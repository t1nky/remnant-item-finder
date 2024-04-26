package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"refinder/remnant"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type ItemProperties struct {
	ZoneID        int32
	ID            int32
	ParentQuestID int32
}

type ItemComponents struct {
	LootSpawns      interface{}
	Zone            interface{}
	POI             interface{}
	Rewards         []interface{}
	QuestObjectives []interface{}
}

type ItemData struct {
	Name             string
	Properties       ItemProperties
	Components       ItemComponents
	OwnedByCharacter bool
	Quantity         int32
}

type ZoneLinkInfo struct {
	ZoneID          int32
	Label           string
	Type            string
	DestinationLink string
	DestinationZone string
	NameID          string
}

type Event struct {
	Name    string
	Rewards []LootSpawn
}

type ZoneActor struct {
	ID           int32
	ParentZoneID int32
	QuestID      int32
	Label        string
	ZoneLinks    []ZoneLinkInfo
	Events       []Event
	Items        []ItemData
	Children     []*ZoneActor
}

type PersistenceKey struct {
	ContainerKey string
	PersistentID uint64
}

type LootSpawn struct {
	Type             string
	ActorBP          string
	Quantity         int32
	PersistenceKey   PersistenceKey
	OwnedByCharacter bool
}

type CharacterData struct {
	ID        int32
	Archetype string
	Items     []string
	Type      string
}

type ZoneInfo struct {
	ZoneActor *ZoneActor
	Biome     string
	BloodMoon bool
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

func splitByCapital(str string) string {
	re := regexp.MustCompile(`[A-Z][^A-Z]*`)
	splitStr := re.FindAllString(str, -1)
	return strings.TrimSpace(strings.Join(splitStr, " "))
}

func getPrintableName(name string) string {
	if !strings.HasSuffix(name, "_C") {
		return name
	}

	if strings.HasPrefix(name, "Material_") {
		name = strings.TrimSuffix(name, "_C")
		name = strings.TrimPrefix(name, "Material_")
		return splitByCapital(name)
	}

	if strings.HasPrefix(name, "Amulet_") || strings.HasPrefix(name, "Armor_") || strings.HasPrefix(name, "Ring_") || strings.HasPrefix(name, "Weapon_") {
		name = strings.TrimSuffix(name, "_C")
		splitted := strings.Split(name, "_")
		name = fmt.Sprintf("{%s} %s", splitted[0], splitByCapital(strings.Join(splitted[1:], " ")))
		return name
	}

	if strings.HasPrefix(name, "Item_HiddenContainer_Material_Engram_") {
		name = strings.TrimSuffix(name, "_C")
		name = strings.TrimPrefix(name, "Item_HiddenContainer_Material_Engram_")
		return fmt.Sprintf("{Archetype Item} %s", splitByCapital(name))
	}

	if strings.HasPrefix(name, "Quest_") {
		name = strings.TrimSuffix(name, "_C")
		name = strings.TrimPrefix(name, "Quest_")
		splitted := strings.Split(name, "_")

		namePrefix := ""
		if splitted[0] == "Injectable" {
			namePrefix = "{Injectable} "
			splitted = splitted[1:]
		}
		if splitted[0] == "SideD" {
			namePrefix = "{Side Dungeon} "
			splitted = splitted[1:]
		}
		if splitted[0] == "OverworldPOI" {
			namePrefix = "{Point of Interest} "
			splitted = splitted[1:]
		}
		if splitted[0] == "Miniboss" {
			namePrefix = "{Miniboss} "
			splitted = splitted[1:]
		}
		return namePrefix + splitByCapital(strings.Join(splitted, "")) // if this will be the case, apply splitByCapital for each item of `splitted``
	}

	if strings.HasPrefix(name, "GemContainer_") {
		name = strings.TrimSuffix(name, "_C")
		name = strings.TrimPrefix(name, "GemContainer_")
		switch name {
		case "BlueGems":
			return "Blue Relic Fragment"
		case "YellowGems":
			return "Yellow Relic Fragment"
		case "RedGems":
			return "Red Relic Fragment"
		default:
			return splitByCapital(name)
		}
	}

	return name
}

func printTreeWithItems(zone *ZoneActor, indent string) {
	if indent == "" {
		fmt.Println(indent + zone.Label)
	} else {
		fmt.Printf("%s %s\n", indent, zone.Label)
	}
	for _, link := range zone.ZoneLinks {
		if link.Type == "EZoneLinkType::Waypoint" {
			fmt.Printf("%s--- || [Waypoint] %s\n", indent, link.Label)
		}
	}
	for _, item := range zone.Items {
		if strings.HasPrefix(item.Name, "Material_") {
			fmt.Printf("%s--- || [Material] x%d %s\n", indent, item.Quantity, getPrintableName(item.Name))
		} else {
			ownedPrint := "✅"
			if !item.OwnedByCharacter {
				ownedPrint = "❌"
			}
			fmt.Printf("%s--- || [Item] %s x%d %s\n", indent, ownedPrint, item.Quantity, getPrintableName(item.Name))
		}
	}
	for _, event := range zone.Events {
		fmt.Printf("%s--- || [Event] %s\n", indent, getPrintableName(event.Name))
		for _, reward := range event.Rewards {
			ownedPrint := "✅"
			if !reward.OwnedByCharacter {
				ownedPrint = "❌"
			}
			fmt.Printf("%s------ || [Reward] %s x%d %s\n", indent, ownedPrint, reward.Quantity, getPrintableName(reward.ActorBP))
		}
	}

	for _, child := range zone.Children {
		printTreeWithItems(child, indent+"---")
	}
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
					for _, reward := range item.Components.Rewards {
						if _, ok := reward.(map[string]interface{})["Spawns"]; !ok {
							continue
						}
						itemSpawns := reward.(map[string]interface{})["Spawns"].(remnant.ArrayStructProperty).Items
						for _, item := range itemSpawns {
							spawnProperties := item.Value.(map[string]interface{})["SpawnEntry"].(remnant.StructProperty).Value.(map[string]interface{})
							keyProperties := item.Value.(map[string]interface{})["Key"].(remnant.StructProperty).Value.(map[string]interface{})

							actorBP := spawnProperties["ActorBP"].(string)
							actorBPSplit := strings.Split(spawnProperties["ActorBP"].(string), ".")
							if len(actorBPSplit) > 1 {
								actorBP = actorBPSplit[1]
							}

							lootSpawns = append(lootSpawns, LootSpawn{
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
		Biome:     biome,
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

func printCharacter(characterData CharacterData, zoneInfo ZoneInfo) {
	fmt.Print("\033[2J")

	fmt.Printf("%-11s %s\n", "Archetype:", characterData.Archetype)

	characterType := strings.TrimPrefix(characterData.Type, "ERemnantCharacterType::")
	fmt.Printf("%-11s %s\n", "Character:", characterType)
	fmt.Printf("%-11s %s\n", "Biome:", remnant.BiomeNames[zoneInfo.Biome])

	if zoneInfo.Biome == "Jungle" {
		fmt.Printf("%-11s %v\n\n", "Blood Moon:", zoneInfo.BloodMoon)
	}

	printTreeWithItems(zoneInfo.ZoneActor, "")
}

func main() {
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

	printCharacter(characters[activeCharacterID], characterZones[activeCharacterID])

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool) // Create a channel to signal program termination
	// Capture Ctrl+C signal (optional)
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
		done <- true
	}()

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					if !strings.HasSuffix(event.Name, ".sav") {
						return
					}
					fullPath = path.Join(basePath, event.Name)

					if event.Name == "profile.sav" {
						characters, activeCharacterID, err = refreshProfile(fullPath)
						if err != nil {
							log.Fatal(err)
						}
						characterZones[activeCharacterID], err = refreshSaveFile(fullPath, characters[activeCharacterID])
						if err != nil {
							log.Fatal(err)
						}

						printCharacter(characters[activeCharacterID], characterZones[activeCharacterID])
					} else if strings.HasPrefix(event.Name, "save_") {
						characterID, err := strconv.ParseInt(strings.Split(strings.Split(event.Name, "_")[1], ".")[0], 10, 32)
						if err != nil {
							log.Fatal(err)
						}
						characterZones[int32(characterID)], err = refreshSaveFile(fullPath, characters[int32(characterID)])
						if err != nil {
							log.Fatal(err)
						}

						if characterID == int64(activeCharacterID) {
							printCharacter(characters[activeCharacterID], characterZones[activeCharacterID])
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
	<-done
	fmt.Println("Closing application...")
}
