package protocol

type PokedexSpeciesEntry struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Type1       string  `json:"type1"`
	Type2       *string `json:"type2" tstype:"string | null,required"`
	PokedexType *string `json:"pokedexType" tstype:"string | null,required"`
	Height      *string `json:"height" tstype:"string | null,required"`
	Weight      *int    `json:"weight" tstype:"number | null,required"`
	PokedexText *string `json:"pokedexText" tstype:"string | null,required"`
	IconImage   *string `json:"iconImage" tstype:"string | null,required"`
	CrySFX      *string `json:"crySfx,omitempty"`
	CryPitch    *int    `json:"cryPitch,omitempty"`
	CryLength   *int    `json:"cryLength,omitempty"`
}

type PokedexStatusEntry struct {
	PokemonID int  `json:"pokemonId"`
	Seen      bool `json:"seen"`
	Caught    bool `json:"caught"`
}

type TrainerCardResponse struct {
	Success       bool     `json:"success" tstype:"true"`
	Name          string   `json:"name"`
	Money         int      `json:"money"`
	TimePlayed    int      `json:"timePlayed"`
	Badges        []string `json:"badges"`
	BadgeCount    int      `json:"badgeCount"`
	PokedexSeen   int      `json:"pokedexSeen"`
	PokedexCaught int      `json:"pokedexCaught"`
}

// PokedexListResponse publishes complete species and character status arrays.
type PokedexListResponse struct {
	Success bool                  `json:"success" tstype:"true"`
	Species []PokedexSpeciesEntry `json:"species"`
	Status  []PokedexStatusEntry  `json:"status"`
}

type PokedexStatusResponse struct {
	Success bool                 `json:"success" tstype:"true"`
	Status  []PokedexStatusEntry `json:"status"`
}
