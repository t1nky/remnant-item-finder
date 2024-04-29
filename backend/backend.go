package backend

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"refinder/backend/remnant"
	"slices"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ItemProperties struct {
	ZoneID        int32 `json:"zoneId"`
	ID            int32 `json:"id"`
	ParentQuestID int32 `json:"parentQuestId"`
}

type ItemComponents struct {
	LootSpawns      interface{}
	Zone            interface{}
	POI             interface{}
	Rewards         []interface{}
	QuestObjectives []interface{}
}

type ItemData struct {
	Name             string         `json:"name"`
	Properties       ItemProperties `json:"properties"`
	Components       ItemComponents `json:"-"`
	OwnedByCharacter bool           `json:"ownedByCharacter"`
	Quantity         int32          `json:"quantity"`
}

type ZoneLinkInfo struct {
	ZoneID          int32  `json:"zoneId"`
	Label           string `json:"label"`
	Type            string `json:"type"`
	DestinationLink string `json:"destinationLink"`
	DestinationZone string `json:"destinationZone"`
	NameID          string `json:"nameId"`
}

type Event struct {
	Name    string      `json:"name"`
	Rewards []LootSpawn `json:"rewards"`
}

type ZoneActor struct {
	ID           int32          `json:"id"`
	ParentZoneID int32          `json:"parentZoneId"`
	QuestID      int32          `json:"questId"`
	Label        string         `json:"label"`
	ZoneLinks    []ZoneLinkInfo `json:"zoneLinks"`
	Events       []Event        `json:"events"`
	Items        []ItemData     `json:"items"`
	Children     []*ZoneActor   `json:"children"`
}

type PersistenceKey struct {
	ContainerKey string `json:"containerKey"`
	PersistentID uint64 `json:"persistentId"`
}

type LootSpawn struct {
	ID               int            `json:"id"`
	RewardID         int            `json:"rewardId"`
	Type             string         `json:"type"`
	ActorBP          string         `json:"actorBp"`
	Quantity         int32          `json:"quantity"`
	PersistenceKey   PersistenceKey `json:"persistenceKey"`
	OwnedByCharacter bool           `json:"ownedByCharacter"`
}

type CharacterData struct {
	ID        int32    `json:"id"`
	Archetype string   `json:"archetype"`
	Items     []string `json:"items"`
	Type      string   `json:"type"`
}

type ZoneInfo struct {
	ZoneActor *ZoneActor `json:"zoneActor"`
	Biome     string     `json:"biome"`
	BloodMoon bool       `json:"bloodMoon"`
}

func buildTree(zones []ZoneActor) *ZoneActor {
	zoneMap := make(map[int]*ZoneActor)
	for i := range zones {
		zoneMap[int(zones[i].ID)] = &zones[i]
	}

	var root *ZoneActor
	for i := range zones {
		if zones[i].ParentZoneID == 0 {
			var hasLinks bool
			for _, link := range zones[i].ZoneLinks {
				if link.DestinationLink != "None" || link.DestinationZone != "None" {
					hasLinks = true
					break
				}
			}
			if hasLinks {
				root = &zones[i]
			}
		} else if parent, ok := zoneMap[int(zones[i].ParentZoneID)]; ok {
			parent.Children = append(parent.Children, &zones[i])
		}
	}

	return root
}

func getTextPropertyValue(textProperty remnant.TextProperty) string {
	textData, ok := textProperty.Data.(remnant.TextData)
	if ok {
		return textData.Data
	}
	textPropertyData, ok := textProperty.Data.(remnant.TextPropertyData)
	if ok {
		return textPropertyData.SourceString
	}
	return ""
}

func getZoneActor(objects []remnant.UObject) ZoneActor {
	var zoneInfo ZoneActor
	for _, obj := range objects {
		if len(obj.Properties) == 0 {
			continue
		}

		if zoneID, ok := obj.Properties["ID"].(int32); ok {
			zoneInfo.ID = zoneID
		}
		if parentZoneID, ok := obj.Properties["ParentZoneID"].(int32); ok {
			zoneInfo.ParentZoneID = parentZoneID
		}
		if questID, ok := obj.Properties["QuestID"].(int32); ok {
			zoneInfo.QuestID = questID
		}

		zoneInfo.Label = getTextPropertyValue(obj.Properties["Label"].(remnant.TextProperty))

		for _, zoneLink := range obj.Properties["ZoneLinks"].(remnant.ArrayStructProperty).Items {
			zoneLinkValue := zoneLink.Value.(map[string]interface{})
			zoneInfo.ZoneLinks = append(zoneInfo.ZoneLinks, ZoneLinkInfo{
				ZoneID:          zoneLinkValue["ZoneID"].(int32),
				DestinationLink: zoneLinkValue["DestinationLink"].(string),
				DestinationZone: zoneLinkValue["DestinationZone"].(string),
				NameID:          zoneLinkValue["NameID"].(string),
				Label:           getTextPropertyValue(zoneLinkValue["Label"].(remnant.TextProperty)),
				Type:            zoneLinkValue["Type"].(remnant.EnumProperty).EnumValue,
			})
		}
	}

	return zoneInfo
}

func getItemProperties(objects []remnant.UObject) (ItemProperties, error) {
	var itemProperties ItemProperties
	var ok bool

	for _, obj := range objects {
		if len(obj.Properties) == 0 {
			continue
		}

		if zoneID, ok := obj.Properties["ZoneID"]; ok {
			itemProperties.ZoneID, ok = zoneID.(int32)
			if !ok {
				return ItemProperties{}, fmt.Errorf("could not parse zoneID")
			}
		}
		itemProperties.ID, ok = obj.Properties["ID"].(int32)
		if !ok {
			return ItemProperties{}, fmt.Errorf("could not parse ID")
		}
		if parentQuestID, ok := obj.Properties["ParentQuestID"]; ok {
			itemProperties.ParentQuestID, ok = parentQuestID.(int32)
			if !ok {
				return ItemProperties{}, fmt.Errorf("could not parse ParentQuestID")
			}
		}
		break
	}

	return itemProperties, nil
}

func getItemComponents(objects []remnant.UObject) ItemComponents {
	var itemComponents ItemComponents
	for _, obj := range objects {
		for _, comp := range obj.Components {
			if comp.ComponentKey == "Loot" {
				for compPropName, compPropValue := range comp.Properties {
					if compPropName == "Spawns" {
						itemComponents.LootSpawns = compPropValue
					}
				}
			}
			if comp.ComponentKey == "Zone" {
				itemComponents.Zone = comp.Properties
			}
			if comp.ComponentKey == "POI" {
				itemComponents.POI = comp.Properties
			}
			if strings.HasPrefix(comp.ComponentKey, "Reward_") {
				itemComponents.Rewards = append(itemComponents.Rewards, comp.Properties)
			}
			if strings.HasPrefix(comp.ComponentKey, "QuestObjective_") {
				itemComponents.QuestObjectives = append(itemComponents.QuestObjectives, comp.Properties)
			}
		}
	}

	return itemComponents
}

func processItems(items []ItemData, zone ZoneActor, characterItems []string) ([]ItemData, []Event, error) {
	var resultItems []ItemData
	var resultEvents []Event

	for _, item := range items {
		currentZoneID := item.Properties.ZoneID
		if item.Components.Zone != nil {
			componentZoneMap, ok := item.Components.Zone.(map[string]interface{})
			if !ok {
				return nil, nil, fmt.Errorf("could not parse zone")
			}
			currentZoneID, ok = componentZoneMap["ZoneID"].(int32)
			if !ok {
				return nil, nil, fmt.Errorf("could not parse zoneID")
			}
		}

		if currentZoneID == zone.ID {
			if item.Components.LootSpawns != nil {
				// TODO: Actor spawn
				for _, itemProp := range item.Components.LootSpawns.(remnant.ArrayStructProperty).Items {
					itemProps := itemProp.Value.(map[string]interface{})
					spawnPropertyProps := itemProps["SpawnEntry"].(remnant.StructProperty).Value.(map[string]interface{})
					item.Name = strings.Split(spawnPropertyProps["ActorBP"].(string), ".")[1]
					item.Quantity = spawnPropertyProps["Quantity"].(int32)
				}

				if slices.Contains(characterItems, item.Name) {
					item.OwnedByCharacter = true
				}
				resultItems = append(resultItems, item)
			} else {
				var currentEvent Event
				currentEvent.Name = item.Name
				if item.Components.Rewards != nil {
					lootSpawns := []LootSpawn{}
					for rewardIdx, reward := range item.Components.Rewards {
						if _, ok := reward.(map[string]interface{})["Spawns"]; !ok {
							continue
						}
						itemSpawns := reward.(map[string]interface{})["Spawns"].(remnant.ArrayStructProperty).Items
						for idx, item := range itemSpawns {
							spawnProperties := item.Value.(map[string]interface{})["SpawnEntry"].(remnant.StructProperty).Value.(map[string]interface{})
							keyProperties := item.Value.(map[string]interface{})["Key"].(remnant.StructProperty).Value.(map[string]interface{})

							actorBP := spawnProperties["ActorBP"].(string)
							actorBPSplit := strings.Split(spawnProperties["ActorBP"].(string), ".")
							if len(actorBPSplit) > 1 {
								actorBP = actorBPSplit[1]
							}

							if len(actorBP) < 2 {
								actorBP = "Unknown Event Reward"
							}

							lootSpawns = append(lootSpawns, LootSpawn{
								ID:       idx,
								RewardID: rewardIdx,
								Type:     spawnProperties["Type"].(remnant.EnumProperty).EnumValue,
								ActorBP:  actorBP,
								Quantity: spawnProperties["Quantity"].(int32),
								PersistenceKey: PersistenceKey{
									ContainerKey: keyProperties["ContainerKey"].(string),
									PersistentID: keyProperties["PersistentID"].(uint64),
								},
								OwnedByCharacter: slices.Contains(characterItems, actorBP),
							})
						}
					}
					currentEvent.Rewards = lootSpawns
				}
				resultEvents = append(resultEvents, currentEvent)
			}
		}
	}

	return resultItems, resultEvents, nil
}

func findAdventure(result *remnant.SaveArchive, characterItems []string) (ZoneInfo, error) {
	var adventureObject remnant.UObject
	for _, obj := range result.Data.Objects {
		for propName, propValue := range obj.Properties {
			if propName == "Key" && strings.HasSuffix(propValue.(string), "Main.Main:PersistentLevel") {
				adventureObject = obj
				break
			}
		}

		if len(adventureObject.Properties) > 0 {
			break
		}
	}

	if len(adventureObject.Properties) == 0 {
		return ZoneInfo{}, fmt.Errorf("could not find base properties")
	}

	var adventureActor remnant.Actor
	for _, actorValue := range adventureObject.Properties["Blob"].(remnant.StructProperty).Value.(remnant.PersistenceContainer).Actors {
		if strings.HasPrefix(actorValue.DynamicData.ClassPath.Name, "Quest_AdventureMode_") {
			adventureActor = actorValue
			break
		}
	}

	if len(adventureActor.Archive.Objects) == 0 {
		return ZoneInfo{}, fmt.Errorf("could not find adventure actors")
	}

	var id int32
	var ok bool
	for _, obj := range adventureActor.Archive.Objects {
		if id, ok = obj.Properties["ID"].(int32); ok {
			break
		}
	}

	var adventureContainerObject remnant.UObject
	for _, obj := range result.Data.Objects {
		for propName, propValue := range obj.Properties {
			if propName == "Key" && strings.HasPrefix(propValue.(string), fmt.Sprintf("/Game/Quest_%d_Container", id)) {
				adventureContainerObject = obj
			}
		}
	}

	actors := adventureContainerObject.Properties["Blob"].(remnant.StructProperty).Value.(remnant.PersistenceContainer).Actors

	zoneActors := []ZoneActor{}
	items := []ItemData{}

	for _, actor := range actors {
		if strings.HasPrefix(actor.DynamicData.ClassPath.Name, "Quest_Global_") {
			continue
		}
		if actor.DynamicData.ClassPath.Name == "ZoneActor" {
			zoneActors = append(zoneActors, getZoneActor(actor.Archive.Objects))
		} else {
			itemProperties, err := getItemProperties(actor.Archive.Objects)
			if err != nil {
				fmt.Println(err)
				continue
			}
			itemComponents := getItemComponents(actor.Archive.Objects)
			items = append(items, ItemData{
				Name:       actor.DynamicData.ClassPath.Name,
				Properties: itemProperties,
				Components: itemComponents,
			})
		}
	}

	var bloodMoon bool
	for _, archiveObj := range adventureActor.Archive.Objects {
		for _, archiveComp := range archiveObj.Components {
			if archiveComp.ComponentKey == "Variables" {
				vars := archiveComp.Properties["Variables"].(remnant.Variables)
				if _, ok := vars.Properties["IsBloodMoon"]; ok {
					bloodMoon = vars.Properties["IsBloodMoon"].(bool)
					break
				}
			}
		}
	}

	for i, actor := range zoneActors {
		items, events, err := processItems(items, actor, characterItems)
		if err != nil {
			log.Fatal(err)
		}

		actor.Items = items
		actor.Events = events

		zoneActors[i] = actor
	}

	tree := buildTree(zoneActors)

	biome := adventureActor.DynamicData.ClassPath.Name
	biome = strings.TrimPrefix(biome, "Quest_AdventureMode_")
	biome = strings.TrimSuffix(biome, "_C")

	return ZoneInfo{
		ZoneActor: tree,
		BloodMoon: bloodMoon,
		Biome:     remnant.BiomeNames[biome],
	}, nil
}

func refreshSaveFile(fullPath string, characterData CharacterData) (ZoneInfo, error) {
	fileData, err := remnant.ReadData(fullPath)
	if err != nil {
		log.Fatal(err)
	}

	archive, err := remnant.ReadSaveArchive(bytes.NewReader(fileData))
	if err != nil {
		log.Fatal(err)
	}

	return findAdventure(&archive, characterData.Items)
}

func getArchetypeName(archetype string) string {
	archetype = strings.TrimPrefix(archetype, "Archetype_")
	archetype = strings.TrimSuffix(archetype, "_UI_C")
	return archetype
}

func refreshProfile(fullPath string) (map[int32]CharacterData, int32, error) {
	fileData, err := remnant.ReadData(fullPath)
	if err != nil {
		return nil, 0, err
	}

	archive, err := remnant.ReadSaveArchive(bytes.NewReader(fileData))
	if err != nil {
		return nil, 0, err
	}

	activeCharacterID := int32(-1)
	for _, obj := range archive.Data.Objects {
		if obj.LoadedData.Name == "BP_RemnantSaveGameProfile_C" {
			activeCharacter, ok := obj.Properties["ActiveCharacterIndex"]
			if ok {
				activeCharacterID, ok = activeCharacter.(int32)
				if !ok {
					return nil, 0, fmt.Errorf("could not parse active character")
				}
				break
			}
		}
	}

	charactersData := map[int32]CharacterData{}
	for _, obj := range archive.Data.Objects {
		if obj.LoadedData.Name != "SavedCharacter" {
			continue
		}
		characterData := CharacterData{}
		if id, ok := obj.Properties["ID"].(int32); ok {
			characterData.ID = id
		}
		if characterType, ok := obj.Properties["CharacterType"].(remnant.EnumProperty); ok {
			characterData.Type = characterType.EnumValue
		} else {
			characterData.Type = "ERemnantCharacterType::Standard"
		}
		characterData.Archetype = getArchetypeName(strings.Split(obj.Properties["Archetype"].(string), ".")[1]) + " / " + getArchetypeName(strings.Split(obj.Properties["SecondaryArchetype"].(string), ".")[1])
		characterData.Items = []string{}
		for _, characterDataObj := range obj.Properties["CharacterData"].(remnant.StructProperty).Value.(remnant.PersistenceBlob).Archive.Objects {
			if characterDataObj.LoadedData.Name == "Character_Master_Player_C" {
				for _, charcaterComp := range characterDataObj.Components {
					if charcaterComp.ComponentKey == "Inventory" {
						for _, item := range charcaterComp.Properties["Items"].(remnant.ArrayStructProperty).Items {
							characterData.Items = append(characterData.Items, strings.Split(item.Value.(map[string]interface{})["ItemBP"].(remnant.ObjectProperty).ClassName, ".")[1])
						}
					}
				}
				break
			}
		}
		charactersData[characterData.ID] = characterData
	}

	return charactersData, activeCharacterID, nil
}

type CharacterObject struct {
	Character CharacterData `json:"character"`
	Zone      ZoneInfo      `json:"zone"`
}

func sendCharacter(ctx context.Context, characterData CharacterObject) {
	log.Println("sending character")
	runtime.EventsEmit(ctx, "character", characterData)
}

func Start(ctx context.Context) {
	characters := map[int32]CharacterData{}
	activeCharacterID := int32(-1)
	characterZones := map[int32]ZoneInfo{}

	basePath := path.Join(os.Getenv("USERPROFILE"), "Saved Games", "Remnant2", "Steam")
	userFolders, err := os.ReadDir(basePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatal(err)
		}
	}
	if len(userFolders) == 0 {
		basePath = path.Join(os.Getenv("USERPROFILE"), "Saved Games", "Remnant2")
		userFolders, err := os.ReadDir(basePath)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Fatal(err)
			}
		}
		if len(userFolders) == 0 {
			log.Fatal("Could not find user folders")
		}
	}

	basePath = path.Join(basePath, userFolders[0].Name())
	fullPath := path.Join(basePath, "profile.sav")

	characters, activeCharacterID, err = refreshProfile(fullPath)
	if err != nil {
		log.Fatal(err)
	}

	fullPath = path.Join(basePath, fmt.Sprintf("save_%d.sav", activeCharacterID))
	characterZones[activeCharacterID], err = refreshSaveFile(fullPath, characters[activeCharacterID])
	if err != nil {
		log.Fatal(err)
	}

	// printCharacter(characters[activeCharacterID], characterZones[activeCharacterID])
	sendCharacter(ctx, CharacterObject{
		Character: characters[activeCharacterID],
		Zone:      characterZones[activeCharacterID],
	})

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Printf("event: %v", event)
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					if !strings.HasSuffix(event.Name, ".sav") {
						continue
					}

					fullPath := event.Name
					fileName := filepath.Base(event.Name)
					if fileName == "profile.sav" {
						characters, activeCharacterID, err = refreshProfile(fullPath)
						if err != nil {
							log.Println(err)
							continue
						}
						characterZones[activeCharacterID], err = refreshSaveFile(fullPath, characters[activeCharacterID])
						if err != nil {
							log.Println(err)
							continue
						}

						sendCharacter(ctx, CharacterObject{
							Character: characters[activeCharacterID],
							Zone:      characterZones[activeCharacterID],
						})
					} else if strings.HasPrefix(fileName, "save_") {
						characterID, err := strconv.ParseInt(strings.Split(strings.Split(event.Name, "_")[1], ".")[0], 10, 32)
						if err != nil {
							log.Println(err)
							continue
						}
						characterZones[int32(characterID)], err = refreshSaveFile(fullPath, characters[int32(characterID)])
						if err != nil {
							log.Println(err)
							continue
						}

						if characterID == int64(activeCharacterID) {
							sendCharacter(ctx, CharacterObject{
								Character: characters[activeCharacterID],
								Zone:      characterZones[activeCharacterID],
							})
						} else {
							log.Println("inactive character update", characterID, "to", activeCharacterID)
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(basePath)
	if err != nil {
		log.Print(err)
		return
	}

	// Wait for a signal to terminate the program
	<-ctx.Done()
}
