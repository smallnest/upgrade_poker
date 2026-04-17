package main

// Trick represents one round of play (4 players each playing cards)
type Trick struct {
	LeadPlayer PlayerPosition
	Plays      map[PlayerPosition][]Card
	Order      []PlayerPosition // order of play
	trumpSuit  Suit
	level      Rank
}

func NewTrick(leadPlayer PlayerPosition, trumpSuit Suit, level Rank) *Trick {
	return &Trick{
		LeadPlayer: leadPlayer,
		Plays:      make(map[PlayerPosition][]Card),
		Order:      []PlayerPosition{leadPlayer},
		trumpSuit:  trumpSuit,
		level:      level,
	}
}

// AddPlay adds a player's play to the trick
func (t *Trick) AddPlay(player PlayerPosition, cards []Card) {
	t.Plays[player] = cards
	if len(t.Order) == 0 || t.Order[len(t.Order)-1] != player {
		t.Order = append(t.Order, player)
	}
}

// IsComplete checks if all 4 players have played
func (t *Trick) IsComplete() bool {
	return len(t.Plays) == 4
}

// Winner determines the winner of the trick
func (t *Trick) Winner() PlayerPosition {
	if !t.IsComplete() {
		return t.LeadPlayer
	}

	plays := make([][]Card, 4)
	for i, pos := range t.Order {
		plays[i] = t.Plays[pos]
	}

	winnerIdx := DetermineTrickWinner(plays, t.trumpSuit, t.level)
	return t.Order[winnerIdx]
}

// Points calculates total points in the trick
func (t *Trick) Points() int {
	total := 0
	for _, cards := range t.Plays {
		for _, card := range cards {
			total += card.Points()
		}
	}
	return total
}

// LeadSuit returns the effective suit of the lead play
func (t *Trick) LeadSuit() Suit {
	if len(t.Plays) == 0 {
		return SuitSpade
	}
	leadCards := t.Plays[t.LeadPlayer]
	if len(leadCards) == 0 {
		return SuitSpade
	}
	return EffectiveSuit(leadCards[0], t.trumpSuit, t.level)
}

// LeadCards returns the lead play's cards
func (t *Trick) LeadCards() []Card {
	return t.Plays[t.LeadPlayer]
}

// NextPlayer returns the next player who should play
func (t *Trick) NextPlayer() PlayerPosition {
	if len(t.Order) == 0 {
		return t.LeadPlayer
	}
	lastPlayer := t.Order[len(t.Order)-1]
	return lastPlayer.Next()
}

// CurrentPlayerCount returns how many players have played so far
func (t *Trick) PlayerCount() int {
	return len(t.Plays)
}

// IsKillTrick checks if any player killed (毙) the lead
func (t *Trick) IsKillTrick() bool {
	leadCards := t.LeadCards()
	if len(leadCards) == 0 {
		return false
	}

	for _, cards := range t.Plays {
		if IsKillPlay(cards, leadCards, t.trumpSuit, t.level) {
			return true
		}
	}
	return false
}

// DisplayTrick returns a formatted string showing the trick
func (t *Trick) DisplayTrick() string {
	result := ""
	for _, pos := range t.Order {
		cards := t.Plays[pos]
		cardStr := ""
		for _, c := range cards {
			cardStr += c.String() + " "
		}
		result += formatPosition(pos) + ": " + cardStr + "\n"
	}
	return result
}

func formatPosition(pos PlayerPosition) string {
	switch pos {
	case PositionSouth:
		return "南(你)"
	case PositionWest:
		return "西(AI)"
	case PositionNorth:
		return "北(AI)"
	case PositionEast:
		return "东(AI)"
	}
	return "??"
}
