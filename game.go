package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
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
	BottomCards []Card
	TrumpSuit   Suit
	Level       [2]Rank
	Dealer      PlayerPosition
	CurrentBid  *Bid
	Phase       GamePhase

	CurrentTrick *Trick
	TrickCount   int
	TrickWinner  PlayerPosition

	TeamScore [2]int

	rng *rand.Rand

	// TUI communication
	tui       *TUI
	drawOrder []Card // current hand display order (maps index to Card)
}

func NewGame() *Game {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	g := &Game{
		Level: [2]Rank{Rank3, Rank3},
		rng:   rng,
	}

	g.Players[PositionSouth] = NewPlayer(PositionSouth, true)
	g.Players[PositionWest] = NewPlayer(PositionWest, false)
	g.Players[PositionNorth] = NewPlayer(PositionNorth, false)
	g.Players[PositionEast] = NewPlayer(PositionEast, false)

	g.Dealer = PlayerPosition(rng.Intn(4))

	return g
}

func (g *Game) DealerTeam() Team {
	return PlayerTeam(g.Dealer)
}

func (g *Game) DealerLevel() Rank {
	return g.Level[g.DealerTeam()]
}

func (g *Game) OpponentTeam() Team {
	if g.DealerTeam() == Team0 {
		return Team1
	}
	return Team0
}

func (g *Game) Deal() {
	g.Deck = NewDeck()
	ShuffleDeck(g.Deck, g.rng)

	for i := range g.Players {
		g.Players[i].Hand = make([]Card, 0)
	}

	idx := 0
	for round := 0; round < 25; round++ {
		for player := 0; player < 4; player++ {
			g.Players[player].AddCard(g.Deck[idx])
			idx++
		}
	}

	g.BottomCards = make([]Card, 8)
	copy(g.BottomCards, g.Deck[idx:idx+8])
}

// DealAnimated deals cards one by one with animation
// DealAnimated deals cards one by one with animation
// During dealing, if human can bid (亮主), pause and ask
func (g *Game) DealAnimated() {
	g.Deck = NewDeck()
	ShuffleDeck(g.Deck, g.rng)

	for i := range g.Players {
		g.Players[i].Hand = make([]Card, 0)
	}
	g.tui.dealCounts = [4]int{}
	g.tui.SetPhase(UIPhaseDealing)
	g.CurrentBid = nil

	level := g.DealerLevel()
	humanAsked := false

	idx := 0
	for round := 0; round < 25; round++ {
		for player := 0; player < 4; player++ {
			g.Players[player].AddCard(g.Deck[idx])
			g.tui.dealCounts[player]++
			idx++
		}
		g.tui.SleepForRedraw(80 * time.Millisecond)

		// After each round, check if human (South) can bid
		if !humanAsked && round >= 2 {
			human := g.Players[PositionSouth]
			possibleBids := CanBid(human, level)
			var validBids []Bid
			for _, bid := range possibleBids {
				if g.CurrentBid == nil || CanOverrideBid(bid, *g.CurrentBid) {
					validBids = append(validBids, bid)
				}
			}
			if len(validBids) > 0 {
				g.tui.bidOptions = validBids
				g.tui.SetPhase(UIPhaseBidding)

				suitName := "无主"
				if validBids[0].Suit != SuitJoker {
					suitName = validBids[0].Suit.String()
				}
				g.tui.SetMessage(fmt.Sprintf("你可以亮主：%s %s", validBids[0].Type.String(), suitName),
					[]Button{
						{Label: "[B:亮主]", Action: "bid"},
						{Label: "[P:不亮]", Action: "pass"},
					})

				for {
					action := g.tui.WaitForAction()
					if action.Type == "bid" && len(validBids) > 0 {
						bid := validBids[0]
						bid.Player = PositionSouth
						g.CurrentBid = &bid
						humanAsked = true
						break
					} else if action.Type == "pass" {
						humanAsked = true
						break
					}
				}
				g.tui.SetMessage("", nil)
				g.tui.SetPhase(UIPhaseDealing)
			}
		}

		// AI players check for bid during dealing (only override if higher)
		for p := 0; p < 4; p++ {
			pos := PlayerPosition(p)
			if g.Players[pos].IsHuman {
				continue
			}
			bid := g.aiBidSimple(g.Players[pos], func() []Bid {
				possibleBids := CanBid(g.Players[pos], level)
				var vb []Bid
				for _, b := range possibleBids {
					if g.CurrentBid == nil || CanOverrideBid(b, *g.CurrentBid) {
						vb = append(vb, b)
					}
				}
				return vb
			}())
			if bid != nil {
				g.CurrentBid = bid
			}
		}
	}

	g.BottomCards = make([]Card, 8)
	copy(g.BottomCards, g.Deck[idx:idx+8])

	// Brief pause to show final state
	g.tui.SleepForRedraw(300 * time.Millisecond)
}


// RunBiddingPhase handles bidding with TUI
func (g *Game) RunBiddingPhase() {
	level := g.DealerLevel()

	for i := 0; i < 4; i++ {
		pos := PlayerPosition(i)
		player := g.Players[pos]

		possibleBids := CanBid(player, level)
		var validBids []Bid
		for _, bid := range possibleBids {
			if g.CurrentBid == nil || CanOverrideBid(bid, *g.CurrentBid) {
				validBids = append(validBids, bid)
			}
		}

		if len(validBids) == 0 {
			continue
		}

		if player.IsHuman {
			g.tui.bidOptions = validBids
			g.tui.SetPhase(UIPhaseBidding)

			suitName := "无主"
			if validBids[0].Suit != SuitJoker {
				suitName = validBids[0].Suit.String()
			}
			g.tui.SetMessage(fmt.Sprintf("你可以亮主：%s %s\n或其他选择", validBids[0].Type.String(), suitName),
				[]Button{
					{Label: "[B:亮主]", Action: "bid"},
					{Label: "[P:不亮]", Action: "pass"},
				})

			for {
				action := g.tui.WaitForAction()
				if action.Type == "bid" && len(validBids) > 0 {
					bid := validBids[0]
					g.CurrentBid = &bid
					break
				} else if action.Type == "pass" {
					break
				}
			}
			g.tui.SetMessage("", nil)
		} else {
			// AI bidding logic
			bid := g.aiBidSimple(player, validBids)
			if bid != nil {
				g.CurrentBid = bid
			}
		}
	}

	g.TrumpSuit = GetTrumpSuit(g.CurrentBid)

	for _, p := range g.Players {
		p.SortHand(g.TrumpSuit, level)
	}
}

func (g *Game) aiBidSimple(player *Player, validBids []Bid) *Bid {
	for _, bid := range validBids {
		if bid.Type == BidPairJoker {
			return &bid
		}
	}
	for _, bid := range validBids {
		if bid.Type == BidTripleLevel {
			return &bid
		}
	}
	for _, bid := range validBids {
		if bid.Type == BidPairLevel {
			suitCount := player.CountSuit(bid.Suit, bid.Suit, g.DealerLevel())
			if suitCount >= 4 {
				return &bid
			}
		}
	}
	return nil
}

// DiscardBottom handles the dealer picking up and discarding bottom cards
func (g *Game) DiscardBottom() {
	dealer := g.Players[g.Dealer]

	if dealer.IsHuman {
		// Show bottom cards
		g.tui.SetMessage(fmt.Sprintf("底牌：\n%s", cardsToString(g.BottomCards)),
			[]Button{{Label: "[Enter:确认]", Action: "confirm"}})
		g.tui.WaitForAction()
		g.tui.SetMessage("", nil)

		dealer.Hand = append(dealer.Hand, g.BottomCards...)
		dealer.SortHand(g.TrumpSuit, g.DealerLevel())

		// Let human choose 8 cards to discard via TUI
		g.tui.discardCount = 8
		g.tui.SetPhase(UIPhaseDiscard)

		for {
			action := g.tui.WaitForAction()
			if action.Type == "play" && len(action.CardIdx) == 8 {
				var discards []Card
				for _, idx := range action.CardIdx {
					if idx >= 0 && idx < len(g.drawOrder) {
						discards = append(discards, g.drawOrder[idx])
					}
				}
				if len(discards) == 8 {
					dealer.RemoveCards(discards)
					g.BottomCards = discards
					break
				}
			}
		}
		g.tui.SetPhase(UIPhasePlaying)
	} else {
		dealer.Hand = append(dealer.Hand, g.BottomCards...)
		dealer.SortHand(g.TrumpSuit, g.DealerLevel())
		g.BottomCards = aiDiscard(dealer, g.TrumpSuit, g.DealerLevel())
	}

	dealer.SortHand(g.TrumpSuit, g.DealerLevel())
}

// PlayTrickFromLead handles one trick
func (g *Game) PlayTrickFromLead(leadPlayer PlayerPosition) PlayerPosition {
	level := g.DealerLevel()
	trick := NewTrick(leadPlayer, g.TrumpSuit, level)
	g.CurrentTrick = trick

	currentPlayer := leadPlayer
	for i := 0; i < 4; i++ {
		player := g.Players[currentPlayer]

		var cards []Card
		if player.IsHuman {
			g.tui.waitingForHuman = true
			cards = g.humanPlayTUI(player, trick)
			g.tui.waitingForHuman = false
		} else {
			// Show thinking animation for AI
			g.tui.thinkingPos = currentPlayer
			g.tui.thinking = true
			g.tui.SleepForRedraw(600 * time.Millisecond)
			g.tui.thinking = false
			cards = aiPlay(player, trick, g)
		}

		trick.AddPlay(currentPlayer, cards)
		player.RemoveCards(cards)

		// Clear selection after removing cards to prevent stale selection
		if player.IsHuman {
			g.tui.selected = make(map[int]bool)
			g.tui.cursorIdx = 0
		}

		// Pause so player can see each play (shorter for human since they chose)
		if player.IsHuman {
			g.tui.SleepForRedraw(500 * time.Millisecond)
		} else {
			g.tui.SleepForRedraw(1500 * time.Millisecond)
		}

		currentPlayer = currentPlayer.Next()
	}

	winner := trick.Winner()
	g.TrickWinner = winner
	g.TrickCount++

	return winner
}

// humanPlayTUI handles human play via TUI
func (g *Game) humanPlayTUI(player *Player, trick *Trick) []Card {
	level := g.DealerLevel()
	g.tui.SetPhase(UIPhasePlaying)

	// Build other players' hands for 甩牌 validation
	otherHands := make([][]Card, 0, 3)
	for i := range g.Players {
		if i != int(player.Position) {
			otherHands = append(otherHands, g.Players[i].Hand)
		}
	}

	for {
		action := g.tui.WaitForAction()
		if action.Type == "play" && len(action.CardIdx) > 0 {
			var cards []Card
			for _, idx := range action.CardIdx {
				if idx >= 0 && idx < len(g.drawOrder) {
					cards = append(cards, g.drawOrder[idx])
				}
			}

			var leadCards []Card
			if trick.PlayerCount() > 0 {
				leadCards = trick.LeadCards()
			}

			if ValidatePlay(cards, leadCards, player.Hand, otherHands, g.TrumpSuit, level) {
				// If leading with multiple cards (throw), resolve: only throw if all max, otherwise play smallest
				if len(leadCards) == 0 && len(cards) > 1 {
					cards = ResolveThrow(cards, otherHands, g.TrumpSuit, level)
				}
				g.tui.selected = make(map[int]bool)
				return cards
			}
			// Invalid play - show error and reset selection
			g.tui.selected = make(map[int]bool)
			errMsg := g.explainInvalidPlay(cards, leadCards, player.Hand, otherHands, g.TrumpSuit, level)
			g.tui.SetMessage(errMsg, []Button{{Label: "[Enter:确认]", Action: "confirm"}})
			g.tui.WaitForAction()
			g.tui.SetMessage("", nil)
		}
	}
}

// PlayHand plays a complete hand (25 tricks)
func (g *Game) PlayHand() {
	g.TrickCount = 0
	g.TeamScore = [2]int{0, 0}

	leadPlayer := g.Dealer

	for g.TrickCount < 25 {
		g.tui.SetPhase(UIPhasePlaying)
		winner := g.PlayTrickFromLead(leadPlayer)

		points := g.CurrentTrick.Points()
		winnerTeam := PlayerTeam(winner)
		g.TeamScore[winnerTeam] += points

		// Show trick result for 5 seconds then auto-continue
		g.tui.trickWinner = winner
		g.tui.trickPoints = points
		g.tui.SetPhase(UIPhaseWaitTrick)
		g.tui.SleepForRedraw(3 * time.Second)

		// Clear trick display and continue
		g.CurrentTrick = nil
		leadPlayer = winner
	}

	g.HandleBottomScore()
	g.HandleUpgrade()
}

// HandleBottomScore handles the bottom card scoring
func (g *Game) HandleBottomScore() {
	lastWinnerTeam := PlayerTeam(g.TrickWinner)
	dealerTeam := g.DealerTeam()

	if lastWinnerTeam != dealerTeam {
		bottomPoints := 0
		for _, c := range g.BottomCards {
			bottomPoints += c.Points()
		}

		if bottomPoints > 0 {
			multiplier := CalculateBottomMultiplier(g.CurrentTrick.Plays[g.TrickWinner], g.TrumpSuit, g.DealerLevel())
			totalBottom := bottomPoints * multiplier
			g.TeamScore[lastWinnerTeam] += totalBottom
		}
	}
}

// HandleUpgrade determines the upgrade result
func (g *Game) HandleUpgrade() {
	dealerTeam := g.DealerTeam()
	opponentTeam := g.OpponentTeam()
	opponentScore := g.TeamScore[opponentTeam]

	var upgradeTeam Team
	var upgradeCount int
	newDealer := g.Dealer.Next()

	var resultMsg string

	switch {
	case opponentScore == 0:
		upgradeTeam = dealerTeam
		upgradeCount = 3
		resultMsg = "大光！庄家方连升3级！"
	case opponentScore < 40:
		upgradeTeam = dealerTeam
		upgradeCount = 2
		resultMsg = "庄家方连升2级！"
	case opponentScore < 80:
		upgradeTeam = dealerTeam
		upgradeCount = 1
		resultMsg = "庄家方升1级！"
	case opponentScore < 120:
		upgradeCount = 0
		newDealer = g.Dealer.Next()
		resultMsg = "闲家上台！换庄！"
	case opponentScore < 160:
		upgradeTeam = opponentTeam
		upgradeCount = 1
		newDealer = g.Dealer.Next()
		resultMsg = "闲家上台并升1级！"
	case opponentScore < 200:
		upgradeTeam = opponentTeam
		upgradeCount = 2
		newDealer = g.Dealer.Next()
		resultMsg = "闲家上台并连升2级！"
	default:
		upgradeTeam = opponentTeam
		upgradeCount = 2 + (opponentScore-200)/40
		newDealer = g.Dealer.Next()
		resultMsg = fmt.Sprintf("闲家上台并连升%d级！", upgradeCount)
	}

	if upgradeCount > 0 {
		for i := 0; i < upgradeCount; i++ {
			g.Level[upgradeTeam] = NextLevel(g.Level[upgradeTeam])
		}
		resultMsg += fmt.Sprintf("\n%s方升级至 %s", formatTeam(upgradeTeam), LevelDisplayName(g.Level[upgradeTeam]))
	}

	// Show result via TUI
	msg := fmt.Sprintf("本局结束！\n庄家方（%s）得分：%d\n闲家方（%s）得分：%d\n%s\n下一局庄家：%s",
		formatTeam(dealerTeam), g.TeamScore[dealerTeam],
		formatTeam(opponentTeam), opponentScore,
		resultMsg, formatPosition(newDealer))

	if g.Level[Team0] >= RankGameWon {
		msg += "\n\n南北方获胜！"
		g.Phase = PhaseGameOver
	} else if g.Level[Team1] >= RankGameWon {
		msg += "\n\n东西方获胜！"
		g.Phase = PhaseGameOver
	}

	g.tui.SetPhase(UIPhaseHandResult)
	g.tui.SetMessage(msg, nil)
	g.tui.WaitForActionOrTimeout(5 * time.Second)
	g.tui.SetMessage("", nil)

	g.Dealer = newDealer
}

// RunTUI is the main game loop driven by TUI
func (g *Game) RunTUI(tui *TUI) {
	g.tui = tui

	// Wait for start
	g.tui.SetPhase(UIPhaseWelcome)
	g.tui.SetMessage("升级（拖拉机）纸牌游戏\n两副牌 · 2为常主 · 4人对战",
		[]Button{{Label: "[Enter:开始游戏]", Action: "start"}})
	g.tui.WaitForAction()
	g.tui.SetMessage("", nil)

	g.Phase = PhaseDealing

	for g.Phase != PhaseGameOver {
		g.DealAnimated()

		g.tui.SetPhase(UIPhaseBidding)
		g.RunBiddingPhase()

		g.tui.SetPhase(UIPhaseDiscard)
		g.DiscardBottom()

		g.PlayHand()
	}

	// Game over
	g.tui.SetPhase(UIPhaseGameOver)
	g.tui.SetMessage("游戏结束！\n感谢游玩！", []Button{{Label: "[Enter:退出]", Action: "confirm"}})
	g.tui.WaitForAction()
}

func formatTeam(team Team) string {
	if team == Team0 {
		return "南北"
	}
	return "东西"
}

// cardsToString formats cards for display
func cardsToString(cards []Card) string {
	suitGroups := make(map[Suit][]Card)
	for _, c := range cards {
		suitGroups[c.Suit] = append(suitGroups[c.Suit], c)
	}

	// Sort each group
	for suit := range suitGroups {
		sort.Slice(suitGroups[suit], func(i, j int) bool {
			return suitGroups[suit][i].Rank > suitGroups[suit][j].Rank
		})
	}

	result := ""
	suitOrder := []Suit{SuitSpade, SuitHeart, SuitDiamond, SuitClub, SuitJoker}
	for _, suit := range suitOrder {
		if cards, ok := suitGroups[suit]; ok && len(cards) > 0 {
			result += suit.Symbol() + ": "
			for _, c := range cards {
				result += c.Rank.String() + " "
			}
			result += "\n"
		}
	}
	return result
}

// explainInvalidPlay returns a human-readable explanation of why the play is invalid
func (g *Game) explainInvalidPlay(cards []Card, leadCards []Card, hand []Card, allHands [][]Card, trumpSuit Suit, level Rank) string {
	cardStr := cardsToString(cards)
	if len(cards) == 0 {
		return "请选择要出的牌"
	}

	if len(leadCards) == 0 {
		// Leading
		firstSuit := EffectiveSuit(cards[0], trumpSuit, level)
		for _, c := range cards[1:] {
			if EffectiveSuit(c, trumpSuit, level) != firstSuit {
				return fmt.Sprintf("出的牌花色不一致\n%s\n甩牌必须是同一花色", cardStr)
			}
		}
		groups := AnalyzePlay(cards, trumpSuit, level)
		totalCards := 0
		for _, g2 := range groups {
			totalCards += len(g2.Cards)
		}
		if totalCards != len(cards) {
			return fmt.Sprintf("出的牌不是有效组合\n%s\n需要完整的对子或拖拉机", cardStr)
		}
		if len(groups) > 1 {
			var notMax []string
			for _, g2 := range groups {
				if !isMaxGroup(g2, allHands, trumpSuit, level) {
					label := "单牌"
					if g2.IsPair {
						label = "对子"
					} else if g2.IsTractor {
						label = "拖拉机"
					}
					notMax = append(notMax, fmt.Sprintf("%s(%s)", label, cardsToString(g2.Cards)))
				}
			}
			if len(notMax) > 0 {
				return fmt.Sprintf("不能甩牌 - 不是最大的:\n%s\n以下组合可以被压过:\n%s", cardStr, strings.Join(notMax, ", "))
			}
		}
		return fmt.Sprintf("出牌无效\n%s", cardStr)
	}

	// Following
	if len(cards) != len(leadCards) {
		return fmt.Sprintf("出牌数量不对(需出%d张,选了%d张)", len(leadCards), len(cards))
	}
	leadSuit := EffectiveSuit(leadCards[0], trumpSuit, level)
	leadSuitInHand := 0
	for _, c := range hand {
		if EffectiveSuit(c, trumpSuit, level) == leadSuit {
			leadSuitInHand++
		}
	}
	leadSuitPlayed := 0
	for _, c := range cards {
		if EffectiveSuit(c, trumpSuit, level) == leadSuit {
			leadSuitPlayed++
		}
	}
	if leadSuitInHand > 0 && leadSuitPlayed < min(leadSuitInHand, len(leadCards)) {
		return fmt.Sprintf("必须跟出%s花色的牌(手中有%d张)", leadSuit.Symbol(), leadSuitInHand)
	}

	return fmt.Sprintf("出牌无效\n%s", cardStr)
}
