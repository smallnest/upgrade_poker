package upgrade_poker

import (
	"fmt"
	"math/rand"
	"sort"
)

// Suit represents a card suit
type Suit int

const (
	SuitSpade   Suit = iota // 黑桃
	SuitHeart                // 红心
	SuitDiamond              // 方块
	SuitClub                 // 梅花
	SuitJoker                // 王
)

var suitSymbols = map[Suit]string{
	SuitSpade:   "♠",
	SuitHeart:   "♥",
	SuitDiamond: "♦",
	SuitClub:    "♣",
	SuitJoker:   "🃏",
}

var suitNames = map[Suit]string{
	SuitSpade:   "黑桃",
	SuitHeart:   "红心",
	SuitDiamond: "方块",
	SuitClub:    "梅花",
	SuitJoker:   "王",
}

func (s Suit) String() string {
	if name, ok := suitNames[s]; ok {
		return name
	}
	return fmt.Sprintf("Suit(%d)", int(s))
}

func (s Suit) Symbol() string {
	if sym, ok := suitSymbols[s]; ok {
		return sym
	}
	return "?"
}

// Rank represents a card rank (3 through A)
type Rank int

const (
	Rank2  Rank = 2
	Rank3  Rank = 3
	Rank4  Rank = 4
	Rank5  Rank = 5
	Rank6  Rank = 6
	Rank7  Rank = 7
	Rank8  Rank = 8
	Rank9  Rank = 9
	Rank10 Rank = 10
	RankJ  Rank = 11
	RankQ  Rank = 12
	RankK  Rank = 13
	RankA  Rank = 14
	// Game won sentinel (after completing level 2)
	RankGameWon Rank = 100
	// Joker ranks
	RankSmallJoker Rank = 15
	RankBigJoker   Rank = 16
)

var rankSymbols = map[Rank]string{
	Rank2:  "2",
	Rank3:  "3",
	Rank4:  "4",
	Rank5:  "5",
	Rank6:  "6",
	Rank7:  "7",
	Rank8:  "8",
	Rank9:  "9",
	Rank10: "10",
	RankJ:  "J",
	RankQ:  "Q",
	RankK:  "K",
	RankA:  "A",
	RankSmallJoker: "小王",
	RankBigJoker:   "大王",
}

func (r Rank) String() string {
	if sym, ok := rankSymbols[r]; ok {
		return sym
	}
	return fmt.Sprintf("Rank(%d)", int(r))
}

// Card represents a single card
type Card struct {
	Suit  Suit
	Rank  Rank
	Copy  int // 0 or 1 for two-deck distinction
}

func (c Card) String() string {
	if c.Suit == SuitJoker {
		return c.Rank.String()
	}
	return c.Suit.Symbol() + c.Rank.String()
}

// FullName returns a descriptive name for the card
func (c Card) FullName() string {
	if c.Suit == SuitJoker {
		return c.Rank.String()
	}
	return c.Suit.String() + c.Rank.String()
}

// IsJoker returns true if the card is a joker
func (c Card) IsJoker() bool {
	return c.Suit == SuitJoker
}

// Points returns the point value of the card
func (c Card) Points() int {
	switch c.Rank {
	case Rank5:
		return 5
	case Rank10, RankK:
		return 10
	default:
		return 0
	}
}

// NewDeck creates a full two-deck set (108 cards)
func NewDeck() []Card {
	deck := make([]Card, 0, 108)
	suits := []Suit{SuitSpade, SuitHeart, SuitDiamond, SuitClub}
	ranks := []Rank{Rank2, Rank3, Rank4, Rank5, Rank6, Rank7, Rank8, Rank9, Rank10, RankJ, RankQ, RankK, RankA}

	for copy := 0; copy < 2; copy++ {
		for _, suit := range suits {
			for _, rank := range ranks {
				deck = append(deck, Card{Suit: suit, Rank: rank, Copy: copy})
			}
		}
		// Jokers
		deck = append(deck, Card{Suit: SuitJoker, Rank: RankSmallJoker, Copy: copy})
		deck = append(deck, Card{Suit: SuitJoker, Rank: RankBigJoker, Copy: copy})
	}

	return deck
}

// Shuffle randomizes the deck
func ShuffleDeck(deck []Card, rng *rand.Rand) {
	rng.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
}

// SortCards sorts cards by suit then rank (for display)
func SortCards(cards []Card, trumpSuit Suit, level Rank) {
	sort.Slice(cards, func(i, j int) bool {
		return CompareForSort(cards[i], cards[j], trumpSuit, level) < 0
	})
}

// CompareForSort compares two cards for sorting purposes
// Returns -1 if a < b, 0 if equal, 1 if a > b
// Sort order: non-trump suits grouped together (♠♥♦♣), then trump suit, then jokers
// Within each group, sorted by rank descending
func CompareForSort(a, b Card, trumpSuit Suit, level Rank) int {
	aIsTrump := IsTrump(a, trumpSuit, level)
	bIsTrump := IsTrump(b, trumpSuit, level)

	// Non-trump before trump
	if !aIsTrump && bIsTrump {
		return -1
	}
	if aIsTrump && !bIsTrump {
		return 1
	}

	// Both trump: compare by trump order (higher order = displayed last)
	aOrder := TrumpOrder(a, trumpSuit, level)
	bOrder := TrumpOrder(b, trumpSuit, level)
	if aOrder != bOrder {
		if aOrder > bOrder {
			return -1 // higher order = smaller display position
		}
		return 1
	}

	// Same trump order, compare by suit for non-jokers
	if a.Suit != b.Suit {
		// Trump suit last among same-order cards
		if a.Suit == trumpSuit {
			return 1
		}
		if b.Suit == trumpSuit {
			return -1
		}
		if a.Suit < b.Suit {
			return -1
		}
		return 1
	}

	// Same suit and order, compare by rank descending
	if a.Rank != b.Rank {
		if a.Rank > b.Rank {
			return -1
		}
		return 1
	}

	return 0
}

// GroupBySuit groups cards by effective suit for display
// Trump cards (level rank, 2s, trump suit cards, jokers) are grouped under trumpSuit
func GroupBySuit(cards []Card, trumpSuit Suit, level Rank) map[Suit][]Card {
	groups := make(map[Suit][]Card)
	for _, c := range cards {
		s := EffectiveSuit(c, trumpSuit, level)
		groups[s] = append(groups[s], c)
	}
	// Sort each group
	for suit := range groups {
		SortCards(groups[suit], trumpSuit, level)
	}
	return groups
}

// RankLevel converts an integer level (2-14 representing 2-A) to a Rank
func RankLevel(level int) Rank {
	if level >= 3 && level <= 14 {
		return Rank(level)
	}
	if level == 15 {
		return RankA // A level
	}
	return Rank2
}

// LevelFromRank converts a Rank to a level number for game progression
func LevelFromRank(r Rank) int {
	switch r {
	case Rank2:
		return 2
	case Rank3:
		return 3
	case Rank4:
		return 4
	case Rank5:
		return 5
	case Rank6:
		return 6
	case Rank7:
		return 7
	case Rank8:
		return 8
	case Rank9:
		return 9
	case Rank10:
		return 10
	case RankJ:
		return 11
	case RankQ:
		return 12
	case RankK:
		return 13
	case RankA:
		return 14
	default:
		return int(r)
	}
}

// NextLevel returns the next level rank after the given one
// Level progression: 3,4,5,6,7,8,9,10,J,Q,K,A,2 (then game won)
func NextLevel(current Rank) Rank {
	switch current {
	case RankA:
		return Rank2
	case Rank2:
		return RankGameWon
	default:
		return current + 1
	}
}

// LevelDisplayName returns the display name for a level
func LevelDisplayName(level Rank) string {
	return level.String()
}
