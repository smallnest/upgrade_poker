package upgrade_poker

import "fmt"

// BidType represents the type of bid
type BidType int

const (
	BidNone       BidType = iota // No bid
	BidPairLevel                 // 对级牌 (pair of level rank)
	BidTripleLevel               // 三张级牌 (three of level rank)
	BidPairJoker                 // 对王 (pair of jokers, no trump)
)

func (b BidType) String() string {
	switch b {
	case BidNone:
		return "无"
	case BidPairLevel:
		return "对级牌"
	case BidTripleLevel:
		return "三张级牌"
	case BidPairJoker:
		return "对王(无主)"
	default:
		return "未知"
	}
}

// Bid represents a bid made by a player
type Bid struct {
	Type     BidType
	Suit     Suit      // The suit being declared as trump (meaningless for BidPairJoker)
	Player   PlayerPosition
	Priority int // Higher priority overrides lower
}

// BidPriority returns the priority of a bid type
func BidPriority(bidType BidType) int {
	switch bidType {
	case BidPairLevel:
		return 1
	case BidTripleLevel:
		return 2
	case BidPairJoker:
		return 3
	default:
		return 0
	}
}

// CanBid checks if a player can make a bid given their hand
func CanBid(player *Player, level Rank) []Bid {
	var bids []Bid

	// Check for pair of level rank cards
	suitPairCount := make(map[Suit]int) // count of level-rank cards per suit
	for _, c := range player.Hand {
		if c.Rank == level {
			suitPairCount[c.Suit]++
		}
	}

	for suit, count := range suitPairCount {
		if count >= 2 {
			bids = append(bids, Bid{
				Type:     BidPairLevel,
				Suit:     suit,
				Player:   player.Position,
				Priority: BidPriority(BidPairLevel),
			})
		}
		if count >= 3 {
			bids = append(bids, Bid{
				Type:     BidTripleLevel,
				Suit:     suit,
				Player:   player.Position,
				Priority: BidPriority(BidTripleLevel),
			})
		}
	}

	// Check for pair of jokers
	smallJokerCount := 0
	bigJokerCount := 0
	for _, c := range player.Hand {
		if c.Rank == RankSmallJoker {
			smallJokerCount++
		}
		if c.Rank == RankBigJoker {
			bigJokerCount++
		}
	}

	// Pair of same joker type (2 small or 2 big) counts as pair of jokers
	if smallJokerCount >= 2 || bigJokerCount >= 2 {
		bids = append(bids, Bid{
			Type:     BidPairJoker,
			Suit:     SuitJoker,
			Player:   player.Position,
			Priority: BidPriority(BidPairJoker),
		})
	}

	return bids
}

// CanOverrideBid checks if a new bid can override the current bid
func CanOverrideBid(newBid, currentBid Bid) bool {
	return newBid.Priority > currentBid.Priority
}

// BidPhase handles the bidding phase of the game
// Returns the winning bid (or nil if no one bid)
type BidPhase struct {
	players   [4]*Player
	level     Rank
	currentBid *Bid
	bidHistory []Bid
}

func NewBidPhase(players [4]*Player, level Rank) *BidPhase {
	return &BidPhase{
		players: players,
		level:   level,
	}
}

// Run executes the bidding phase
// In the simple rule: go around once, highest bid wins
func (bp *BidPhase) Run() *Bid {
	// Each player gets one chance to bid
	for i := 0; i < 4; i++ {
		pos := PlayerPosition(i)
		player := bp.players[pos]

		possibleBids := CanBid(player, bp.level)

		// Filter out bids that can't override current
		var validBids []Bid
		for _, bid := range possibleBids {
			if bp.currentBid == nil || CanOverrideBid(bid, *bp.currentBid) {
				validBids = append(validBids, bid)
			}
		}

		if len(validBids) == 0 {
			if player.IsHuman {
				fmt.Printf("你没有可以亮主的牌。\n")
			}
			continue
		}

		if player.IsHuman {
			// Let human choose
			bp.humanBid(player, validBids)
		} else {
			// AI decides
			bp.aiBid(player, validBids)
		}
	}

	return bp.currentBid
}

func (bp *BidPhase) humanBid(player *Player, validBids []Bid) {
	fmt.Printf("\n你可以亮主：\n")
	for i, bid := range validBids {
		suitName := "无主"
		if bid.Suit != SuitJoker {
			suitName = bid.Suit.String()
		}
		fmt.Printf("  %d. %s %s\n", i+1, bid.Type.String(), suitName)
	}
	fmt.Printf("  0. 不亮\n")
	fmt.Printf("请选择：")

	var choice int
	fmt.Scanln(&choice)

	if choice >= 1 && choice <= len(validBids) {
		bid := validBids[choice-1]
		bp.currentBid = &bid
		bp.bidHistory = append(bp.bidHistory, bid)
		suitName := "无主"
		if bid.Suit != SuitJoker {
			suitName = bid.Suit.String()
		}
		fmt.Printf("你亮主：%s %s\n", bid.Type.String(), suitName)
	}
}

func (bp *BidPhase) aiBid(player *Player, validBids []Bid) {
	// AI strategy: bid if hand is strong in that suit
	// Simple: bid the highest priority available if the suit has enough cards
	for _, bid := range validBids {
		if bid.Type == BidPairJoker {
			// Always bid no trump with pair of jokers
			bp.currentBid = &bid
			bp.bidHistory = append(bp.bidHistory, bid)
			fmt.Printf("%s 亮主：对王(无主)\n", formatPosition(player.Position))
			return
		}
	}

	// For pair/triple of level rank, bid if we have enough cards in that suit
	for _, bid := range validBids {
		if bid.Type == BidTripleLevel {
			// Triple is strong, always bid
			bp.currentBid = &bid
			bp.bidHistory = append(bp.bidHistory, bid)
			fmt.Printf("%s 亮主：%s %s\n", formatPosition(player.Position), bid.Type.String(), bid.Suit.String())
			return
		}
	}

	// For pair, bid if we have 4+ cards of that suit
	for _, bid := range validBids {
		if bid.Type == BidPairLevel {
			suitCount := player.CountSuit(bid.Suit, bid.Suit, bp.level)
			if suitCount >= 4 {
				bp.currentBid = &bid
				bp.bidHistory = append(bp.bidHistory, bid)
				fmt.Printf("%s 亮主：%s %s\n", formatPosition(player.Position), bid.Type.String(), bid.Suit.String())
				return
			}
		}
	}

	// No bid
}

// GetTrumpSuit returns the trump suit from the bid result
func GetTrumpSuit(bid *Bid) Suit {
	if bid == nil {
		return SuitJoker // No bid = no trump
	}
	if bid.Type == BidPairJoker {
		return SuitJoker // No trump
	}
	return bid.Suit
}
