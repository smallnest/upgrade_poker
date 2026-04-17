package main

import "fmt"

// PlayerPosition represents the seating position
type PlayerPosition int

const (
	PositionSouth PlayerPosition = iota // 南 (human)
	PositionWest                        // 西 (AI)
	PositionNorth                       // 北 (AI)
	PositionEast                        // 东 (AI)
)

var positionNames = map[PlayerPosition]string{
	PositionSouth: "南(你)",
	PositionWest:  "西(AI)",
	PositionNorth: "北(AI)",
	PositionEast:  "东(AI)",
}

func (p PlayerPosition) String() string {
	return positionNames[p]
}

func (p PlayerPosition) Partner() PlayerPosition {
	return (p + 2) % 4
}

func (p PlayerPosition) Next() PlayerPosition {
	return (p + 1) % 4
}

// Team represents a team (0 or 1)
type Team int

const (
	Team0 Team = 0 // South+North
	Team1 Team = 1 // West+East
)

func PlayerTeam(pos PlayerPosition) Team {
	if pos == PositionSouth || pos == PositionNorth {
		return Team0
	}
	return Team1
}

// Player represents a game player
type Player struct {
	Position PlayerPosition
	Hand     []Card
	IsHuman  bool
	Name     string
}

func NewPlayer(pos PlayerPosition, isHuman bool) *Player {
	return &Player{
		Position: pos,
		IsHuman:  isHuman,
		Hand:     make([]Card, 0),
		Name:     positionNames[pos],
	}
}

// AddCard adds a card to the player's hand
func (p *Player) AddCard(card Card) {
	p.Hand = append(p.Hand, card)
}

// RemoveCards removes specific cards from the player's hand
func (p *Player) RemoveCards(cards []Card) {
	toRemove := make(map[Card]bool)
	for _, c := range cards {
		toRemove[c] = true
	}

	newHand := make([]Card, 0, len(p.Hand))
	for _, c := range p.Hand {
		if !toRemove[c] {
			newHand = append(newHand, c)
		} else {
			delete(toRemove, c) // Only remove one copy
		}
	}
	p.Hand = newHand
}

// HasCard checks if the player has a specific card
func (p *Player) HasCard(card Card) bool {
	for _, c := range p.Hand {
		if c == card {
			return true
		}
	}
	return false
}

// HasSuit checks if the player has any card of the given effective suit
func (p *Player) HasSuit(suit Suit, trumpSuit Suit, level Rank) bool {
	for _, c := range p.Hand {
		if EffectiveSuit(c, trumpSuit, level) == suit {
			return true
		}
	}
	return false
}

// CountSuit counts cards of the given effective suit
func (p *Player) CountSuit(suit Suit, trumpSuit Suit, level Rank) int {
	count := 0
	for _, c := range p.Hand {
		if EffectiveSuit(c, trumpSuit, level) == suit {
			count++
		}
	}
	return count
}

// GetCardsOfSuit returns all cards of the given effective suit
func (p *Player) GetCardsOfSuit(suit Suit, trumpSuit Suit, level Rank) []Card {
	var result []Card
	for _, c := range p.Hand {
		if EffectiveSuit(c, trumpSuit, level) == suit {
			result = append(result, c)
		}
	}
	return result
}

// CountRank counts how many cards of the given rank (in any suit)
func (p *Player) CountRank(rank Rank) int {
	count := 0
	for _, c := range p.Hand {
		if c.Rank == rank {
			count++
		}
	}
	return count
}

// GetCardsOfRank returns cards of the given rank
func (p *Player) GetCardsOfRank(rank Rank) []Card {
	var result []Card
	for _, c := range p.Hand {
		if c.Rank == rank {
			result = append(result, c)
		}
	}
	return result
}

// SortHand sorts the player's hand
func (p *Player) SortHand(trumpSuit Suit, level Rank) {
	SortCards(p.Hand, trumpSuit, level)
}

// HandSize returns the number of cards in hand
func (p *Player) HandSize() int {
	return len(p.Hand)
}

// DisplayHand returns a formatted string of the player's hand
func (p *Player) DisplayHand(trumpSuit Suit, level Rank) string {
	p.SortHand(trumpSuit, level)
	groups := GroupBySuit(p.Hand, trumpSuit, level)

	result := ""

	// Display trump suit first
	if trumpSuit != SuitJoker {
		if cards, ok := groups[trumpSuit]; ok && len(cards) > 0 {
			result += fmt.Sprintf("  %s(主): ", trumpSuit.Symbol())
			for _, c := range cards {
				if c.IsJoker() {
					result += c.String() + " "
				} else {
					result += c.Rank.String() + " "
				}
			}
			result += "\n"
		}
	}

	// Display non-trump suits
	suitOrder := []Suit{SuitSpade, SuitHeart, SuitDiamond, SuitClub}
	for _, suit := range suitOrder {
		if suit == trumpSuit {
			continue
		}
		if cards, ok := groups[suit]; ok && len(cards) > 0 {
			result += fmt.Sprintf("  %s: ", suit.Symbol())
			for _, c := range cards {
				result += c.Rank.String() + " "
			}
			result += "\n"
		}
	}

	// Display jokers (if no trump suit - no trump game)
	if trumpSuit == SuitJoker {
		if cards, ok := groups[SuitJoker]; ok && len(cards) > 0 {
			result += "  王: "
			for _, c := range cards {
				result += c.String() + " "
			}
			result += "\n"
		}
	}

	return result
}
