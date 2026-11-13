package protocol

type PhaserPokemonDataRequest struct {
	PokemonID int `json:"pokemonId"`
}

type PhaserMoveDataRequest struct {
	MoveID int `json:"moveId"`
}

type PhaserItemDataRequest struct {
	ItemID int `json:"itemId"`
}

type PhaserPokemonFull struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	HP               int     `json:"hp"`
	Atk              int     `json:"atk"`
	Def              int     `json:"def"`
	Spd              int     `json:"spd"`
	Spc              int     `json:"spc"`
	Type1            string  `json:"type1"`
	Type2            *string `json:"type2" tstype:"string | null,required"`
	CatchRate        int     `json:"catchRate"`
	BaseExp          int     `json:"baseExp"`
	DefaultMove1     *string `json:"defaultMove1Id" tstype:"string | null,required"`
	DefaultMove2     *string `json:"defaultMove2Id" tstype:"string | null,required"`
	DefaultMove3     *string `json:"defaultMove3Id" tstype:"string | null,required"`
	DefaultMove4     *string `json:"defaultMove4Id" tstype:"string | null,required"`
	BaseCry          *int    `json:"baseCry" tstype:"number | null,required"`
	CryPitch         *int    `json:"cryPitch" tstype:"number | null,required"`
	CryLength        *int    `json:"cryLength" tstype:"number | null,required"`
	PokedexType      *string `json:"pokedexType" tstype:"string | null,required"`
	Height           *string `json:"height" tstype:"string | null,required"`
	Weight           *int    `json:"weight" tstype:"number | null,required"`
	PokedexText      *string `json:"pokedexText" tstype:"string | null,required"`
	EvolveLevel      *int    `json:"evolveLevel" tstype:"number | null,required"`
	EvolvePokemon    *string `json:"evolvePokemon" tstype:"string | null,required"`
	EvolvesFromTrade *int    `json:"evolvesFromTrade" tstype:"number | null,required"`
	IconImage        *string `json:"iconImage" tstype:"string | null,required"`
	PaletteType      *string `json:"paletteType" tstype:"string | null,required"`
}

type PhaserMoveFull struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	ShortName       string  `json:"shortName"`
	Effect          *string `json:"effect" tstype:"string | null,required"`
	Power           *int    `json:"power" tstype:"number | null,required"`
	Type            *string `json:"type" tstype:"string | null,required"`
	Accuracy        *int    `json:"accuracy" tstype:"number | null,required"`
	PP              *int    `json:"pp" tstype:"number | null,required"`
	BattleAnimation *string `json:"battleAnimation" tstype:"string | null,required"`
	BattleSound     *string `json:"battleSound" tstype:"string | null,required"`
	IsHM            int     `json:"isHm"`
	FieldMoveEffect *int    `json:"fieldMoveEffect" tstype:"number | null,required"`
}

type PhaserItemFull struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	ShortName     string `json:"shortName"`
	Price         *int   `json:"price" tstype:"number | null,required"`
	IsUsable      int    `json:"isUsable"`
	UsesPartyMenu int    `json:"usesPartyMenu"`
	VendingPrice  *int   `json:"vendingPrice" tstype:"number | null,required"`
	MoveID        *int   `json:"moveId" tstype:"number | null,required"`
	IsGuardDrink  int    `json:"isGuardDrink"`
	IsKeyItem     int    `json:"isKeyItem"`
}

type PhaserPokemonDataResponse struct {
	PhaserPokemonFull `tstype:",extends"`
	Success           bool `json:"success" tstype:"true"`
}

type PhaserMoveDataResponse struct {
	PhaserMoveFull `tstype:",extends"`
	Success        bool `json:"success" tstype:"true"`
}

type PhaserItemDataResponse struct {
	PhaserItemFull `tstype:",extends"`
	Success        bool `json:"success" tstype:"true"`
}
