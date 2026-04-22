#!/usr/bin/env python3
"""
Convert a Google My Maps KML export into seed nodes.csv and pipes.csv
for the hydraulic model pipeline.

Usage:
    python3 kml2csv.py map-features.kml

Outputs:
    nodes.csv   — junctions, tanks, reservoirs with lat/lon
    pipes.csv   — pipe segments with from/to node references

The CSVs will have geometry filled in from the KML. You still need
to fill in diameter_in, material, elevation_ft, demand_gpm, and
account by hand (or with other scripts).

How it works:
    - KML Point placemarks become JUNCTION nodes (or RESERVOIR if
      they use the dark-blue infrastructure style).
    - KML LineString placemarks become sequences of pipe segments.
      Each vertex on a line becomes a JUNCTION node. Consecutive
      vertices are connected by pipe segments.
    - Each service connection point is connected to the nearest
      line vertex with a service pipe.
    - Nodes within 30 feet of each other are merged (a line vertex
      near a service connection becomes one node).
"""

import sys
import csv
import math
from xml.etree import ElementTree as ET

NS = "http://www.opengis.net/kml/2.2"
MERGE_THRESHOLD_FT = 30


def haversine_ft(lon1, lat1, lon2, lat2):
    """Distance between two WGS84 points in feet."""
    R = 20902231  # earth radius in feet
    p1, p2 = math.radians(lat1), math.radians(lat2)
    dp = math.radians(lat2 - lat1)
    dl = math.radians(lon2 - lon1)
    a = math.sin(dp / 2) ** 2 + math.cos(p1) * math.cos(p2) * math.sin(dl / 2) ** 2
    return R * 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a))


def slug(name):
    """Turn a placemark name into a usable node ID."""
    return (
        name.replace(" ", "-")
        .replace(",", "")
        .replace("#", "")
        .replace("'", "")
    )


def parse_kml(path):
    """Parse KML into lists of points and lines."""
    tree = ET.parse(path)
    points = []
    lines = []

    for pm in tree.iter(f"{{{NS}}}Placemark"):
        name = pm.findtext(f"{{{NS}}}name", "").strip()
        desc = pm.findtext(f"{{{NS}}}description", "").strip()
        style = pm.findtext(f"{{{NS}}}styleUrl", "").strip()

        pt = pm.find(f".//{{{NS}}}Point/{{{NS}}}coordinates")
        ls = pm.find(f".//{{{NS}}}LineString/{{{NS}}}coordinates")

        if pt is not None:
            parts = pt.text.strip().split(",")
            lon, lat = float(parts[0]), float(parts[1])
            # Dark blue style (1A237E) = infrastructure (well, tank)
            is_infra = "1A237E" in style
            points.append((name, lon, lat, is_infra))

        elif ls is not None:
            coords = []
            for c in ls.text.strip().split():
                p = c.split(",")
                coords.append((float(p[0]), float(p[1])))
            lines.append((name, desc, coords))

    return points, lines


def find_nearby(nodes, lon, lat):
    """Find an existing node within MERGE_THRESHOLD_FT, or None."""
    for n in nodes:
        if haversine_ft(lon, lat, n["lon"], n["lat"]) < MERGE_THRESHOLD_FT:
            return n["id"]
    return None


def build_nodes_and_pipes(points, lines):
    nodes = []
    pipes = []
    pipe_counter = 1

    # --- Infrastructure points (wells, tanks) ---
    for name, lon, lat, infra in points:
        if not infra:
            continue
        nodes.append({
            "id": slug(name),
            "type": "RESERVOIR",
            "lat": lat,
            "lon": lon,
            "elevation_ft": "",
            "demand_gpm": "0",
            "static_psi": "",
            "account": "",
            "address": "",
            "notes": "Well site",
        })

    # --- Line vertices as junction nodes + pipe segments ---
    line_vertex_ids = set()
    line_seqs = []

    for line_name, desc, coords in lines:
        seq = []
        for i, (lon, lat) in enumerate(coords):
            nearby = find_nearby(nodes, lon, lat)
            if nearby:
                seq.append(nearby)
                line_vertex_ids.add(nearby)
                continue
            nid = f"{slug(line_name)}-V{i + 1}"
            nodes.append({
                "id": nid,
                "type": "JUNCTION",
                "lat": lat,
                "lon": lon,
                "elevation_ft": "",
                "demand_gpm": "0",
                "static_psi": "",
                "account": "",
                "address": "",
                "notes": f"{line_name} vertex",
            })
            seq.append(nid)
            line_vertex_ids.add(nid)
        line_seqs.append((line_name, desc, seq))

    # Pipe segments along each line
    for line_name, desc, seq in line_seqs:
        for i in range(len(seq) - 1):
            mat = "PVC" if "PVC" in (desc or "") else ""
            note = line_name
            if desc and i == 0:
                note += f" ({desc})"
            pipes.append({
                "id": f"P-{pipe_counter:03d}",
                "from_node": seq[i],
                "to_node": seq[i + 1],
                "diameter_in": "",
                "material": mat,
                "roughness": "",
                "install_year": "",
                "notes": note,
            })
            pipe_counter += 1

    # --- Service connection points ---
    for name, lon, lat, infra in points:
        if infra:
            continue
        nearby = find_nearby(nodes, lon, lat)
        if nearby:
            # Merge: update existing vertex with connection info
            for n in nodes:
                if n["id"] == nearby:
                    n["address"] = name
                    n["demand_gpm"] = ""
                    n["notes"] = f"Service connection (merged with line vertex)"
                    break
            continue
        nid = slug(name)
        nodes.append({
            "id": nid,
            "type": "JUNCTION",
            "lat": lat,
            "lon": lon,
            "elevation_ft": "",
            "demand_gpm": "",
            "static_psi": "",
            "account": "",
            "address": name,
            "notes": "Service connection",
        })

    # --- Service pipes: connect each unattached connection to nearest line vertex ---
    vtx_nodes = [n for n in nodes if n["id"] in line_vertex_ids]
    svc_nodes = [n for n in nodes if n["id"] not in line_vertex_ids and n["type"] == "JUNCTION"]

    for sn in svc_nodes:
        best_dist = float("inf")
        best_vtx = None
        for vn in vtx_nodes:
            d = haversine_ft(sn["lon"], sn["lat"], vn["lon"], vn["lat"])
            if d < best_dist:
                best_dist = d
                best_vtx = vn
        if best_vtx and best_dist < 2000:
            pipes.append({
                "id": f"SVC-{pipe_counter:03d}",
                "from_node": best_vtx["id"],
                "to_node": sn["id"],
                "diameter_in": "",
                "material": "",
                "roughness": "",
                "install_year": "",
                "notes": f"Service line to {sn['address']} ({best_dist:.0f} ft)",
            })
            pipe_counter += 1

    return nodes, pipes, line_vertex_ids, svc_nodes


def write_csvs(nodes, pipes):
    node_fields = [
        "id", "type", "lat", "lon", "elevation_ft", "demand_gpm",
        "static_psi", "account", "address", "notes",
    ]
    pipe_fields = [
        "id", "from_node", "to_node", "diameter_in", "material",
        "roughness", "install_year", "notes",
    ]

    with open("model/nodes.csv", "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=node_fields)
        w.writeheader()
        w.writerows(nodes)

    with open("model/pipes.csv", "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=pipe_fields)
        w.writeheader()
        w.writerows(pipes)


def print_summary(nodes, pipes, line_vertex_ids, svc_nodes):
    reservoirs = sum(1 for n in nodes if n["type"] == "RESERVOIR")
    vtx = len(line_vertex_ids)
    svc = len(svc_nodes)
    main_pipes = len(pipes) - svc

    print(f"nodes.csv: {len(nodes)} nodes "
          f"({reservoirs} reservoir, {vtx} line vertices, {svc} service connections)")
    print(f"pipes.csv: {len(pipes)} pipe segments "
          f"({main_pipes} main + {svc} service)")

    print("\n--- Nodes ---")
    for n in nodes:
        addr = f"  addr: {n['address']}" if n["address"] else ""
        print(f"  {n['id']:35s}  {n['type']:10s}  "
              f"({n['lat']:.7f}, {n['lon']:.7f}){addr}")

    print("\n--- Pipes ---")
    node_map = {n["id"]: n for n in nodes}
    for p in pipes:
        fn = node_map[p["from_node"]]
        tn = node_map[p["to_node"]]
        length = haversine_ft(fn["lon"], fn["lat"], tn["lon"], tn["lat"])
        print(f"  {p['id']:8s}  {p['from_node']:30s} -> "
              f"{p['to_node']:30s}  {length:6.0f} ft  {p['notes']}")


def main():
    kml_path = sys.argv[1] if len(sys.argv) > 1 else "model/map-features.kml"
    points, lines = parse_kml(kml_path)

    print(f"Parsed {len(points)} points and {len(lines)} lines from {kml_path}\n")

    nodes, pipes, vtx_ids, svc_nodes = build_nodes_and_pipes(points, lines)
    write_csvs(nodes, pipes)
    print_summary(nodes, pipes, vtx_ids, svc_nodes)

    print(f"\nWrote model/nodes.csv and model/pipes.csv")
    print("Fill in: diameter_in, material, elevation_ft, demand_gpm, account")


if __name__ == "__main__":
    main()
