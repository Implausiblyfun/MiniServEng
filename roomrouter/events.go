package roomrouter

// A PlayEvent is when a user has played a card.
type PlayEvent struct {
	Cp           ConnectionParameters `json:"parameters"`
	Card         interface{}          `json:"card"`
	PlayLocation int                  `json:"play_location"`
}

// A DestroyEvent is when a played card has been destroyed
type DestroyEvent struct {
	Cp            ConnectionParameters `json:"parameters"`
	IsOpponent    bool                 `json:"is_opponent"`
	PlayLocation  int                  `json:"play_location"`
	IsEnchantment bool                 `json:"is_enchantment"`
}

// CardChangeEvent is when a played card has been modified
type CardChangeEvent struct {
	// Todo: if cards get expensive in size (they shouldn't),
	// we could send a delta instead
	Cp               ConnectionParameters `json:"parameters"`
	PositionEncoding string               `json:"opponent"`
	Card             interface{}          `json:"card"`
}

// A PullEvent is when a user has pulled some number of cards.
type PullEvent struct {
	Cp       ConnectionParameters `json:"parameters"`
	Count    int                  `json:"count"`
	DeckSize int                  `json:"deck_size"`
}

// A ShuffleEvent is when a user has shuffled their deck some number of times.
type ShuffleEvent struct {
	Cp ConnectionParameters `json:"parameters"`
}

// A PlayerConnectedEvent is fired on the first connect of other players.
type PlayerConnectedEvent struct {
	Cp ConnectionParameters `json:"parameters"`
}

// A EndTurnEvent signifies when a player has finished their turn
type EndTurnEvent struct {
	Cp ConnectionParameters `json:"parameters"`
	// Game state should be here, once the server
	// is deciding game logic
}

// A GameEndEvent is sent when a player concedes or loses a game
type GameEndEvent struct {
	Cp ConnectionParameters `json:"parameters"`
}

// AreYouStillThereEvent will be sent to the client if you havent heard from them for a while.
// The silly person is probably still there and just not answering.
type AreYouStillThereEvent struct {
	Cp ConnectionParameters `json:"parameters"`
}

// An Event encapsulates any type of msg we need to send between game instances.
type Event struct {
	Name    string      `json:"name"`
	Payload interface{} `json:"payload"`
}

// ConnectionParameters is the set of parameters all events should have in their payloads.
type ConnectionParameters struct {
	PlayerID string `json:"player_id"`
}

// BlameStringStatic is a placeholder we can use until we can get BlameString working on all events.
const BlameStringStatic = "Got an invalid event from the wire"
