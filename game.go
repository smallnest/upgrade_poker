package main

import (
	"fmt"
	"math/rand"
	"time"
)

// GamePhase represents the current phase of the game
type GamePhase int

const (
	PhaseDealing GamePhase = iota
	PhaseBidding
	PhaseDiscarding
	PhasePlaying
	PhaseScoring
	PhaseGameOver
)

// Game represents the overall game state
type Game struct {
	Players     [4]*Player
	Deck        []Card
	BottomCards []Card // 8 cards on the bottom
	TrumpSuit   Suit
	Level       [2]Rank // Level for each team (Team0, Team1)
	Dealer      PlayerPosition
	CurrentBid  *Bid
	Phase       GamePhase

	// Trick state
	CurrentTrick *Trick
	TrickCount   int
	TrickWinner  PlayerPosition

	// Scoring
	TeamScore [2]int // Points collected by each team's opponent (off-score)

	// Random
	rng *rand.Rand

	// UI
	inputChan chan string
}

func NewGame() *Game {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	g := &Game{
		Level: [2]Rank{Rank2, Rank2}, // Both teams start at level 2
		rng:   rng,
	}

	// Create players
	g.Players[PositionSouth] = NewPlayer(PositionSouth, true)
	g.Players[PositionWest] = NewPlayer(PositionWest, false)
	g.Players[PositionNorth] = NewPlayer(PositionNorth, false)
	g.Players[PositionEast] = NewPlayer(PositionEast, false)

	// Random first dealer
	g.Dealer = PlayerPosition(rng.Intn(4))

	return g
}

// DealerTeam returns which team is the dealer's team
func (g *Game) DealerTeam() Team {
	return PlayerTeam(g.Dealer)
}

// DealerLevel returns the current level the dealer is playing
func (g *Game) DealerLevel() Rank {
	return g.Level[g.DealerTeam()]
}

// OpponentTeam returns the team that is not the dealer
func (g *Game) OpponentTeam() Team {
	if g.DealerTeam() == Team0 {
		return Team1
	}
	return Team0
}

// Deal shuffles and deals the cards
func (g *Game) Deal() {
	g.Deck = NewDeck()
	ShuffleDeck(g.Deck, g.rng)

	// Reset player hands
	for i := range g.Players {
		g.Players[i].Hand = make([]Card, 0)
	}

	// Deal 25 cards to each player
	idx := 0
	for round := 0; round < 25; round++ {
		for player := 0; player < 4; player++ {
			g.Players[player].AddCard(g.Deck[idx])
			idx++
		}
	}

	// Remaining 8 cards are the bottom
	g.BottomCards = make([]Card, 8)
	copy(g.BottomCards, g.Deck[idx:idx+8])
}

// RunBiddingPhase handles the bidding phase
func (g *Game) RunBiddingPhase() {
	level := g.DealerLevel()
	bidPhase := NewBidPhase(g.Players, level)
	g.CurrentBid = bidPhase.Run()
	g.TrumpSuit = GetTrumpSuit(g.CurrentBid)

	if g.CurrentBid != nil {
		// The winning bidder becomes the new dealer
		g.Dealer = g.CurrentBid.Player
	} else {
		fmt.Println("\n无人亮主，打无主！")
	}

	// Sort hands after trump is determined
	for _, p := range g.Players {
		p.SortHand(g.TrumpSuit, level)
	}
}

// DiscardBottom handles the dealer picking up and discarding bottom cards
func (g *Game) DiscardBottom() {
	dealer := g.Players[g.Dealer]

	if dealer.IsHuman {
		// Show bottom cards
		fmt.Printf("\n底牌：")
		for _, c := range g.BottomCards {
			fmt.Printf("%s ", c.String())
		}
		fmt.Println()

		// Add bottom cards to hand
		dealer.Hand = append(dealer.Hand, g.BottomCards...)
		dealer.SortHand(g.TrumpSuit, g.DealerLevel())

		// Let human choose 8 cards to discard
		fmt.Printf("\n你的手牌（含底牌，共%d张）：\n", len(dealer.Hand))
		fmt.Println(dealer.DisplayHand(g.TrumpSuit, g.DealerLevel()))

		g.BottomCards = g.humanDiscard(dealer)
	} else {
		// AI: add bottom cards and discard weakest 8
		dealer.Hand = append(dealer.Hand, g.BottomCards...)
		dealer.SortHand(g.TrumpSuit, g.DealerLevel())
		g.BottomCards = aiDiscard(dealer, g.TrumpSuit, g.DealerLevel())
	}

	dealer.SortHand(g.TrumpSuit, g.DealerLevel())
}

func (g *Game) humanDiscard(dealer *Player) []Card {
	for {
		level := g.DealerLevel()
		dealer.SortHand(g.TrumpSuit, level)
		groups := GroupBySuit(dealer.Hand, g.TrumpSuit, level)
		indexedCards := make([]Card, 0, len(dealer.Hand))
		indexedCards = append(indexedCards, Card{}) // index 0 unused

		fmt.Printf("请选择8张牌扣底（输入牌的序号，空格分隔）：\n")

		// Trump suit first
		if g.TrumpSuit != SuitJoker {
			if cards, ok := groups[g.TrumpSuit]; ok && len(cards) > 0 {
				fmt.Printf("  %s(主): ", g.TrumpSuit.Symbol())
				for _, c := range cards {
					indexedCards = append(indexedCards, c)
					fmt.Printf("%d:%s ", len(indexedCards)-1, c.Rank.String())
				}
				fmt.Println()
			}
		}
		suitOrder := []Suit{SuitSpade, SuitHeart, SuitDiamond, SuitClub}
		for _, suit := range suitOrder {
			if suit == g.TrumpSuit {
				continue
			}
			if cards, ok := groups[suit]; ok && len(cards) > 0 {
				fmt.Printf("  %s: ", suit.Symbol())
				for _, c := range cards {
					indexedCards = append(indexedCards, c)
					fmt.Printf("%d:%s ", len(indexedCards)-1, c.Rank.String())
				}
				fmt.Println()
			}
		}
		if g.TrumpSuit == SuitJoker {
			if cards, ok := groups[SuitJoker]; ok && len(cards) > 0 {
				fmt.Printf("  王: ")
				for _, c := range cards {
					indexedCards = append(indexedCards, c)
					fmt.Printf("%d:%s ", len(indexedCards)-1, c.String())
				}
				fmt.Println()
			}
		}
		fmt.Println()

		var input string
		fmt.Scanln(&input)

		indices := parseIndices(input, len(indexedCards)-1)
		if len(indices) != 8 {
			fmt.Printf("需要选择8张牌，你选择了%d张。请重试。\n", len(indices))
			continue
		}

		// Check for duplicate and valid indices
		seen := make(map[int]bool)
		valid := true
		for _, idx := range indices {
			if idx < 0 || idx >= len(indexedCards) || seen[idx] {
				valid = false
				break
			}
			seen[idx] = true
		}
		if !valid {
			fmt.Println("选择无效，请重新选择。")
			continue
		}

		var discards []Card
		for _, idx := range indices {
			discards = append(discards, indexedCards[idx])
		}

		dealer.RemoveCards(discards)
		fmt.Printf("扣底：")
		for _, c := range discards {
			fmt.Printf("%s ", c.String())
		}
		fmt.Println()

		return discards
	}
}

// parseIndices parses space-separated 1-based indices from input
func parseIndices(input string, max int) []int {
	var result []int
	num := 0
	hasNum := false

	for _, ch := range input {
		if ch >= '0' && ch <= '9' {
			num = num*10 + int(ch-'0')
			hasNum = true
		} else if hasNum {
			if num >= 1 && num <= max {
				result = append(result, num-1) // Convert to 0-based
			}
			num = 0
			hasNum = false
		}
	}
	if hasNum && num >= 1 && num <= max {
		result = append(result, num-1)
	}

	return result
}

// PlayTrickFromLead handles one trick of play starting from the given lead player
func (g *Game) PlayTrickFromLead(leadPlayer PlayerPosition) PlayerPosition {
	level := g.DealerLevel()
	trick := NewTrick(leadPlayer, g.TrumpSuit, level)
	g.CurrentTrick = trick

	// Each player plays in order
	currentPlayer := leadPlayer
	for i := 0; i < 4; i++ {
		player := g.Players[currentPlayer]

		var cards []Card
		if player.IsHuman {
			cards = g.humanPlay(player, trick)
		} else {
			cards = aiPlay(player, trick, g)
		}

		trick.AddPlay(currentPlayer, cards)
		player.RemoveCards(cards)

		currentPlayer = currentPlayer.Next()
	}

	// Determine winner
	winner := trick.Winner()
	g.TrickWinner = winner
	g.TrickCount++

	return winner
}

func (g *Game) humanPlay(player *Player, trick *Trick) []Card {
	level := g.DealerLevel()

	for {
		fmt.Printf("\n--- 第%d轮出牌 ---\n", g.TrickCount+1)
		if trick.PlayerCount() > 0 {
			fmt.Println("已出的牌：")
			fmt.Print(trick.DisplayTrick())
		}

		// Build indexed card list grouped by suit
		player.SortHand(g.TrumpSuit, level)
		groups := GroupBySuit(player.Hand, g.TrumpSuit, level)
		indexedCards := make([]Card, 0, len(player.Hand)) // 1-based index mapping
		indexedCards = append(indexedCards, Card{})        // index 0 unused

		fmt.Printf("\n你的手牌（%d张）：\n", len(player.Hand))

		// Trump suit first
		if g.TrumpSuit != SuitJoker {
			if cards, ok := groups[g.TrumpSuit]; ok && len(cards) > 0 {
				fmt.Printf("  %s(主): ", g.TrumpSuit.Symbol())
				for _, c := range cards {
					indexedCards = append(indexedCards, c)
					fmt.Printf("%d:%s ", len(indexedCards)-1, c.Rank.String())
				}
				fmt.Println()
			}
		}
		// Non-trump suits
		suitOrder := []Suit{SuitSpade, SuitHeart, SuitDiamond, SuitClub}
		for _, suit := range suitOrder {
			if suit == g.TrumpSuit {
				continue
			}
			if cards, ok := groups[suit]; ok && len(cards) > 0 {
				fmt.Printf("  %s: ", suit.Symbol())
				for _, c := range cards {
					indexedCards = append(indexedCards, c)
					fmt.Printf("%d:%s ", len(indexedCards)-1, c.Rank.String())
				}
				fmt.Println()
			}
		}
		// Jokers for no-trump game
		if g.TrumpSuit == SuitJoker {
			if cards, ok := groups[SuitJoker]; ok && len(cards) > 0 {
				fmt.Printf("  王: ")
				for _, c := range cards {
					indexedCards = append(indexedCards, c)
					fmt.Printf("%d:%s ", len(indexedCards)-1, c.String())
				}
				fmt.Println()
			}
		}

		// Show which cards to play
		if trick.PlayerCount() == 0 {
			fmt.Printf("你领出，请选择要出的牌（输入序号，空格分隔）：\n")
		} else {
			leadSuit := trick.LeadSuit()
			suitName := "主牌"
			if leadSuit != g.TrumpSuit || g.TrumpSuit == SuitJoker {
				suitName = leadSuit.String()
			}
			fmt.Printf("跟牌（%s，需出%d张），请选择要出的牌（输入序号，空格分隔）：\n", suitName, len(trick.LeadCards()))
		}

		var input string
		fmt.Scanln(&input)

		indices := parseIndices(input, len(indexedCards)-1)
		if len(indices) == 0 {
			fmt.Println("请选择至少一张牌。")
			continue
		}

		// Check for duplicate indices and out of range
		seen := make(map[int]bool)
		valid := true
		for _, idx := range indices {
			if idx < 0 || idx >= len(indexedCards) {
				valid = false
				break
			}
			if seen[idx] {
				valid = false
				break
			}
			seen[idx] = true
		}
		if !valid {
			fmt.Println("选择无效，请重新选择。")
			continue
		}

		var cards []Card
		for _, idx := range indices {
			cards = append(cards, indexedCards[idx])
		}

		// Validate the play
		var leadCards []Card
		if trick.PlayerCount() > 0 {
			leadCards = trick.LeadCards()
		}

		if !ValidatePlay(cards, leadCards, player.Hand, g.TrumpSuit, level) {
			fmt.Println("出牌不合法，请重新选择。")
			continue
		}

		return cards
	}
}

// PlayHand plays a complete hand (25 tricks)
func (g *Game) PlayHand() {
	g.TrickCount = 0
	g.TeamScore = [2]int{0, 0}

	leadPlayer := g.Dealer

	for g.TrickCount < 25 {
		winner := g.PlayTrickFromLead(leadPlayer)

		// Add trick points to the winning team
		points := g.CurrentTrick.Points()
		winnerTeam := PlayerTeam(winner)
		g.TeamScore[winnerTeam] += points

		// Show trick result
		fmt.Printf("\n%s 赢得此轮，获得%d分\n", formatPosition(winner), points)

		leadPlayer = winner
	}

	// Handle 抠底 (bottom card scoring)
	g.HandleBottomScore()

	// Calculate upgrade
	g.HandleUpgrade()
}

// HandleBottomScore handles the bottom card scoring (抠底)
func (g *Game) HandleBottomScore() {
	// If the last trick was won by the opponent team, they get the bottom card points
	lastWinnerTeam := PlayerTeam(g.TrickWinner)
	dealerTeam := g.DealerTeam()

	if lastWinnerTeam != dealerTeam {
		// Opponent won the last trick - 抠底
		bottomPoints := 0
		for _, c := range g.BottomCards {
			bottomPoints += c.Points()
		}

		if bottomPoints > 0 {
			multiplier := CalculateBottomMultiplier(g.CurrentTrick.Plays[g.TrickWinner], g.TrumpSuit, g.DealerLevel())
			totalBottom := bottomPoints * multiplier
			g.TeamScore[lastWinnerTeam] += totalBottom
			fmt.Printf("\n抠底！底牌%d分 × %d倍 = %d分，%s方获得\n",
				bottomPoints, multiplier, totalBottom,
				formatTeam(lastWinnerTeam))
		}
	}
}

// HandleUpgrade determines the upgrade result
func (g *Game) HandleUpgrade() {
	dealerTeam := g.DealerTeam()
	opponentTeam := g.OpponentTeam()

	// The "score" for upgrade purposes is how many points the OPPONENT collected
	// (闲家得分)
	opponentScore := g.TeamScore[opponentTeam]

	fmt.Printf("\n========================================\n")
	fmt.Printf("本局结束！\n")
	fmt.Printf("庄家方（%s）得分：%d\n", formatTeam(dealerTeam), g.TeamScore[dealerTeam])
	fmt.Printf("闲家方（%s）得分：%d\n", formatTeam(opponentTeam), opponentScore)
	fmt.Printf("========================================\n")

	var upgradeTeam Team
	var upgradeCount int
	newDealer := g.Dealer.Next() // Default: opponent takes over

	switch {
	case opponentScore == 0:
		// 大光：庄家升3级
		upgradeTeam = dealerTeam
		upgradeCount = 3
		fmt.Println("大光！庄家方连升3级！")
	case opponentScore < 40:
		// 庄家升2级
		upgradeTeam = dealerTeam
		upgradeCount = 2
		fmt.Println("庄家方连升2级！")
	case opponentScore < 80:
		// 庄家升1级
		upgradeTeam = dealerTeam
		upgradeCount = 1
		fmt.Println("庄家方升1级！")
	case opponentScore < 120:
		// 闲家上台
		upgradeCount = 0
		newDealer = g.Dealer.Next()
		fmt.Println("闲家上台！换庄！")
	case opponentScore < 160:
		// 闲家上台+升1级
		upgradeTeam = opponentTeam
		upgradeCount = 1
		newDealer = g.Dealer.Next()
		fmt.Println("闲家上台并升1级！")
	case opponentScore < 200:
		// 闲家上台+升2级
		upgradeTeam = opponentTeam
		upgradeCount = 2
		newDealer = g.Dealer.Next()
		fmt.Println("闲家上台并连升2级！")
	default:
		// 闲家上台+升N级
		upgradeTeam = opponentTeam
		upgradeCount = 2 + (opponentScore-200)/40
		newDealer = g.Dealer.Next()
		fmt.Printf("闲家上台并连升%d级！\n", upgradeCount)
	}

	if upgradeCount > 0 {
		for i := 0; i < upgradeCount; i++ {
			g.Level[upgradeTeam] = NextLevel(g.Level[upgradeTeam])
		}
		fmt.Printf("%s方升级至 %s\n", formatTeam(upgradeTeam), LevelDisplayName(g.Level[upgradeTeam]))
	}

	// Check for game over
	if g.Level[Team0] >= RankA {
		fmt.Println("\n🎉 南北方获胜！")
		g.Phase = PhaseGameOver
	} else if g.Level[Team1] >= RankA {
		fmt.Println("\n🎉 东西方获胜！")
		g.Phase = PhaseGameOver
	}

	// Set new dealer
	g.Dealer = newDealer

	fmt.Printf("下一局庄家：%s\n", formatPosition(g.Dealer))
}

func formatTeam(team Team) string {
	if team == Team0 {
		return "南北"
	}
	return "东西"
}

// Run is the main game loop
func (g *Game) Run() {
	fmt.Println("╔══════════════════════════════╗")
	fmt.Println("║    升级（拖拉机）纸牌游戏     ║")
	fmt.Println("║    两副牌 · 2为常主           ║")
	fmt.Println("╚══════════════════════════════╝")
	fmt.Println()

	g.Phase = PhaseDealing

	for g.Phase != PhaseGameOver {
		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("庄家：%s  庄家方打：%s  闲家方打：%s\n",
			formatPosition(g.Dealer),
			LevelDisplayName(g.Level[g.DealerTeam()]),
			LevelDisplayName(g.Level[g.OpponentTeam()]))
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		// Deal
		g.Deal()
		fmt.Println("发牌完毕！")
		g.showHands()

		// Bid
		g.RunBiddingPhase()
		if g.TrumpSuit != SuitJoker {
			fmt.Printf("主牌花色：%s\n", g.TrumpSuit.String())
		} else {
			fmt.Println("无主！")
		}

		// Re-show hands with trump info
		g.showHands()

		// Discard bottom
		g.DiscardBottom()

		// Play
		g.PlayHand()

		if g.Phase != PhaseGameOver {
			fmt.Printf("\n按回车键开始下一局...")
			var discard string
			fmt.Scanln(&discard)
		}
	}

	fmt.Println("\n游戏结束！")
}

// showHands displays the human player's hand
func (g *Game) showHands() {
	level := g.DealerLevel()
	human := g.Players[PositionSouth]
	fmt.Printf("\n你的手牌（%d张）：\n", len(human.Hand))
	fmt.Println(human.DisplayHand(g.TrumpSuit, level))
}
