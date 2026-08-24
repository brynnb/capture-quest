package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMarkOrphanedBlockedCarpetWarpsPostgres(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (
			id INTEGER PRIMARY KEY,
			name TEXT,
			is_overworld INTEGER
		)`,
		`CREATE TABLE phaser_tiles (
			x INTEGER,
			y INTEGER,
			map_id INTEGER,
			collision_type INTEGER
		)`,
		`CREATE TABLE phaser_warps (
			id INTEGER PRIMARY KEY,
			source_map_id INTEGER,
			x INTEGER,
			y INTEGER,
			destination_map_id INTEGER,
			destination_x INTEGER,
			destination_y INTEGER,
			warp_type TEXT,
			warp_direction TEXT
		)`,
		`INSERT INTO phaser_maps (id, name, is_overworld) VALUES
			(6, 'CELADON_CITY', 1),
			(17, 'ROUTE_6', 1),
			(21, 'ROUTE_10', 1),
			(73, 'ROUTE_6_GATE', 0),
			(82, 'ROCK_TUNNEL_1F', 0),
			(136, 'CELADON_MART_5F', 0)`,
		`INSERT INTO phaser_tiles (x, y, map_id, collision_type) VALUES
			(139, -107, NULL, 0),
			(189, -89, NULL, 0),
			(15, 33, 82, 1),
			(15, 35, 82, 0),
			(1, 1, 136, 1)`,
		`INSERT INTO phaser_warps (
			id, source_map_id, x, y, destination_map_id, destination_x, destination_y, warp_type, warp_direction
		) VALUES
			(111, 6, 139, -107, 136, 12, 1, 'carpet', 'UP'),
			(649, 17, 189, -89, 73, 3, 0, 'carpet', 'RIGHT'),
			(483, 73, 3, 0, 17, 190, -89, 'carpet', 'DOWN'),
			(154, 82, 15, 33, 21, 278, -137, 'door', NULL),
			(155, 82, 15, 35, 21, 278, -137, 'carpet', 'DOWN'),
			(900, 6, 140, -107, 136, 5, 5, 'door', NULL),
			(901, 136, 1, 1, 6, 120, -100, 'carpet', 'DOWN')`,
	)

	if err := markOrphanedBlockedCarpetWarpsPostgres(db); err != nil {
		t.Fatalf("markOrphanedBlockedCarpetWarpsPostgres: %v", err)
	}

	rows, err := db.Query(`SELECT id, warp_type, COALESCE(warp_direction, '') FROM phaser_warps ORDER BY id`)
	if err != nil {
		t.Fatalf("query warps: %v", err)
	}
	defer rows.Close()

	got := map[int]struct {
		warpType  string
		direction string
	}{}
	for rows.Next() {
		var id int
		var warpType, direction string
		if err := rows.Scan(&id, &warpType, &direction); err != nil {
			t.Fatalf("scan warp: %v", err)
		}
		got[id] = struct {
			warpType  string
			direction string
		}{warpType: warpType, direction: direction}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read warps: %v", err)
	}

	if got[111].warpType != "inactive" || got[111].direction != "" {
		t.Fatalf("orphaned Celadon wall warp = %#v, want inactive with no direction", got[111])
	}
	if got[649].warpType != "carpet" || got[649].direction != "RIGHT" {
		t.Fatalf("gate warp with inverse evidence changed to %#v, want carpet RIGHT", got[649])
	}
	if got[155].warpType != "carpet" || got[155].direction != "DOWN" {
		t.Fatalf("paired Rock Tunnel edge warp changed to %#v, want carpet DOWN", got[155])
	}
	if got[900].warpType != "door" {
		t.Fatalf("blocked door warp changed to %#v, want door", got[900])
	}
	if got[901].warpType != "carpet" || got[901].direction != "DOWN" {
		t.Fatalf("walkable carpet warp changed to %#v, want carpet DOWN", got[901])
	}
}
