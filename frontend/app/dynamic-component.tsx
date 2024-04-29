"use client";
// DynamicComponent.js
import React, { useCallback, useEffect, useId, useState } from "react";
import { backend } from "@/wailsjs/wailsjs/go/models";
import * as runtime from "@/wailsjs/runtime";
import { CalendarIcon, MoonIcon } from "@heroicons/react/20/solid";
import { CubeIcon } from "@heroicons/react/24/outline";
import Image from "next/image";
import WaypointTree from "./_components/waypoint-tree";

function CharacterComponent({
  character,
  zone,
}: {
  character: backend.CharacterData;
  zone: backend.ZoneInfo;
}) {
  return (
    <div className="bg-gray-800 text-white flex w-72 grow-0 flex-col p-4 rounded-lg shadow space-y-2">
      <h2 className="text-xl font-bold">Character Details</h2>
      <div className="w-full border-t border-gray-600" />
      <div>
        <h3>Archetype</h3>
        <p className="text-gray-400">{character.archetype}</p>
      </div>
      <div>
        <h3>Character</h3>
        <p className="text-gray-400">
          {character.type.replace("ERemnantCharacterType::", "")}
        </p>
      </div>
      <div>
        <h3>Biome</h3>
        <p className="text-gray-400">
          {zone.biome}
          {zone.biome === "Yaesha" &&
            (zone.bloodMoon ? (
              <MoonIcon className="w-5 h-5 text-pink-700" />
            ) : (
              <MoonIcon className="w-5 h-5 text-gray-400" />
            ))}
        </p>
      </div>
    </div>
  );
}

function getPrintableName(name: string) {
  if (!name.endsWith("_C")) {
    return name;
  }

  if (name.startsWith("Material_")) {
    name = name.replace(/^Material_/, "").replace(/_C$/, "");
    return splitByCapital(name);
  }

  if (
    name.startsWith("Amulet_") ||
    name.startsWith("Armor_") ||
    name.startsWith("Ring_") ||
    name.startsWith("Weapon_")
  ) {
    name = name
      .replace("Amulet_Amulet_", "Amulet ")
      .replace("Armor_Armor_", "Armor ")
      .replace("Ring_Ring_", "Ring ")
      .replace("Weapon_Weapon_", "Weapon ")
      .replace(/_C$/, "")
      .replaceAll("_", " ");
    name = splitByCapital(name);
    return `${name}`;
  }

  if (name.startsWith("Item_HiddenContainer_Material_Engram_")) {
    name = name
      .replace("Item_HiddenContainer_Material_Engram_", "")
      .replace(/_C$/, "");
    return `{Archetype Item} ${splitByCapital(name)}`;
  }

  if (name.startsWith("Quest_")) {
    name = name.replace(/^Quest_/, "").replace(/_C$/, "");
    let splitted = name.split("_");
    let nameSuffix = "";
    if (splitted[0] === "Injectable") {
      nameSuffix = " (Injectable)";
      splitted = splitted.slice(1);
    } else if (splitted[0] === "SideD") {
      nameSuffix = " (Side Dungeon)";
      splitted = splitted.slice(1);
    } else if (splitted[0] === "OverworldPOI") {
      nameSuffix = " (Point of Interest)";
      splitted = splitted.slice(1);
    } else if (splitted[0] === "Miniboss") {
      nameSuffix = " (Miniboss)";
      splitted = splitted.slice(1);
    }
    return splitByCapital(splitted.join(" ")) + nameSuffix;
  }

  if (name.startsWith("Char_")) {
    name = name
      .replace(/^Char_/, "")
      .replace(/_C$/, "")
      .replaceAll("_", " ");
    return splitByCapital(name);
  }

  if (name.startsWith("Character_")) {
    name = name
      .replace(/^Character_/, "")
      .replace(/_C$/, "")
      .replaceAll("_", " ");
    return splitByCapital(name);
  }

  if (name.startsWith("GemContainer_")) {
    name = name.replace(/^GemContainer_/, "").replace(/_C$/, "");
    switch (name) {
      case "BlueGems":
        return "Blue Relic Fragment";
      case "YellowGems":
        return "Yellow Relic Fragment";
      case "RedGems":
        return "Red Relic Fragment";
      default:
        return splitByCapital(name);
    }
  }
  return name;
}

function splitByCapital(str: string) {
  return str
    .replace(/[A-Z][^A-Z]*/g, (match) =>
      match.length > 1 ? match + " " : match
    )
    .trim();
}

function ItemComponent({ item }: { item: backend.ItemData }) {
  return (
    <div className="flex justify-between items-center bg-gray-700 text-white p-2 rounded-md text-sm">
      <span>
        {getPrintableName(item.name)} x{item.quantity}
      </span>
      <span>{item.ownedByCharacter ? "✅" : "❌"}</span>
    </div>
  );
}

function EventComponent({ event }: { event: backend.Event }) {
  return (
    <div className="bg-gray-700 text-white p-2 rounded-md space-y-1 text-sm">
      <h3 className="font-semibold">{getPrintableName(event.name)}</h3>
      {!!event.rewards && (
        <ul className="list-disc list-inside pl-2">
          {event.rewards?.map((reward) => (
            <li key={reward.actorBp} className="justify-between relative">
              <span>
                {getPrintableName(reward.actorBp)} x{reward.quantity}
              </span>
              <span className="absolute right-0 top-0">
                {reward.ownedByCharacter ? "✅" : "❌"}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function ZoneComponent({
  zone,
  depth = 0,
}: {
  zone: backend.ZoneActor;
  depth?: number;
}) {
  return (
    <div className={`space-y-2`}>
      <h2
        className={`text-lg font-bold sticky bg-zinc-800 py-0.5 px-2 rounded-lg`}
        style={{ top: depth * 30, zIndex: 1000 - depth }}
      >
        {zone.label}
      </h2>
      <div className="space-y-2 pl-3 relative ">
        {!!zone.zoneLinks && (
          <div className="border shadow p-3 m-2 rounded-md border-bg-zinc-800 space-y-2">
            <div className="flex gap-1">
              <Image
                src="/andrew-stifter-rem2-waycrystal-01.webp"
                width={13}
                height={24}
                alt="Waypoint"
                className="object-cover"
              />
              <h3>Waypoints</h3>
            </div>
            <WaypointTree>
              {zone.zoneLinks?.map((link) => (
                <WaypointTree.Item nested key={link.label}>
                  {link.label ||
                    link.destinationLink ||
                    link.destinationZone ||
                    link.nameId}{" "}
                  ({link.destinationLink})
                </WaypointTree.Item>
              ))}
            </WaypointTree>
          </div>
        )}
        {!!zone.items && (
          <div className="border shadow p-3 m-2 rounded-md border-bg-zinc-800 space-y-2">
            <div className="flex gap-1 pb-2">
              <CubeIcon className="w-6 h-6 text-red-400" />
              <h3>Items</h3>
            </div>
            <div className="space-y-2">
              {zone.items.map((item) => (
                <ItemComponent key={item.name} item={item} />
              )) ?? "-"}
            </div>
          </div>
        )}
        {!!zone.events && (
          <div className="border shadow p-3 m-2 rounded-md border-bg-zinc-800 space-y-2">
            <div className="flex gap-1 pb-2">
              <CalendarIcon className="w-6 h-6 text-red-400" />
              <h3>Events</h3>
            </div>
            {zone.events.map((event) => (
              <EventComponent key={event.name} event={event} />
            )) ?? "-"}
          </div>
        )}
        {
          <div
            id={"children-zones"}
            className={`space-y-2 pl-1 before:absolute before:rounded-md before:top-0 before:bottom-0 before:left-0 before:w-2 before:bg-zinc-800`}
          >
            {zone.children?.map((child) => (
              <ZoneComponent key={child.label} zone={child} depth={depth + 1} />
            ))}
          </div>
        }
      </div>
    </div>
  );
}

export default function DynamicComponent() {
  const [charData, setCharData] = useState<{
    character: backend.CharacterData;
    zone: backend.ZoneInfo;
  } | null>(null);

  const onCharacter = useCallback((data: any) => {
    console.log("character", data);
    setCharData(data);
  }, []);

  useEffect(() => {
    runtime.EventsOn("character", onCharacter);
    return () => runtime.EventsOff("character");
  }, [onCharacter]);

  return (
    <div className="flex flex-row gap-4 w-full p-4">
      {charData && (
        <CharacterComponent
          character={charData.character}
          zone={charData.zone}
        />
      )}
      <div className="w-full">
        <div className="bg-black text-white rounded-lg">
          {charData?.zone.zoneActor && (
            <ZoneComponent zone={charData.zone.zoneActor} />
          )}
        </div>
      </div>
    </div>
  );
}
