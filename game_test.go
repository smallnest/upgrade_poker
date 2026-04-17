package main

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestNewDeck(t *testing.T) {
	deck := NewDeck()
	if len(deck) != 108 {
		t.Errorf("Expected 108 cards, got %d", len(deck))
	}
}

func TestDeal(t *testing.T) {
	g := NewGame()
	g.Deal()

	totalCards := 0
	for _, p := range g.Players {
		if len(p.Hand) != 25 {
			t.Errorf("Expected 25 cards per player, got %d", len(p.Hand))
		}
		totalCards += len(p.Hand)
	}
	totalCards += len(g.BottomCards)
	if totalCards != 108 {
		t.Errorf("Expected 108 total cards, got %d", totalCards)
	}
	if len(g.BottomCards) != 8 {
		t.Errorf("Expected 8 bottom cards, got %d", len(g.BottomCards))
	}
}

func TestTrumpDetection(t *testing.T) {
	// When playing level 10, heart is trump
	level := Rank10
	trumpSuit := SuitHeart

	// Big joker is always trump
	if !IsTrump(Card{Suit: SuitJoker, Rank: RankBigJoker}, trumpSuit, level) {
		t.Error("Big joker should be trump")
	}
	// Small joker is always trump
	if !IsTrump(Card{Suit: SuitJoker, Rank: RankSmallJoker}, trumpSuit, level) {
		t.Error("Small joker should be trump")
	}
	// Level rank of any suit is trump
	if !IsTrump(Card{Suit: SuitSpade, Rank: Rank10}, trumpSuit, level) {
		t.Error("Level rank of any suit should be trump")
	}
	// 2 is 常主, always trump
	if !IsTrump(Card{Suit: SuitClub, Rank: Rank2}, trumpSuit, level) {
		t.Error("2 should be trump (常主)")
	}
	// Trump suit cards are trump
	if !IsTrump(Card{Suit: SuitHeart, Rank: Rank7}, trumpSuit, level) {
		t.Error("Trump suit cards should be trump")
	}
	// Non-trump suit non-level non-2 card is not trump
	if IsTrump(Card{Suit: SuitSpade, Rank: Rank7}, trumpSuit, level) {
		t.Error("Non-trump suit non-level non-2 card should not be trump")
	}
}

func TestTrumpOrder(t *testing.T) {
	level := Rank10
	trumpSuit := SuitHeart

	bigJoker := Card{Suit: SuitJoker, Rank: RankBigJoker}
	smallJoker := Card{Suit: SuitJoker, Rank: RankSmallJoker}
	mainLevel := Card{Suit: SuitHeart, Rank: Rank10}
	offLevel := Card{Suit: SuitSpade, Rank: Rank10}
	main2 := Card{Suit: SuitHeart, Rank: Rank2}
	off2 := Card{Suit: SuitSpade, Rank: Rank2}
	trumpA := Card{Suit: SuitHeart, Rank: RankA}

	cases := []struct {
		a, b     Card
		expected int // 1 if a > b
	}{
		{bigJoker, smallJoker, 1},
		{smallJoker, mainLevel, 1},
		{mainLevel, offLevel, 1},
		{offLevel, main2, 1},
		{main2, off2, 1},
		{off2, trumpA, 1},
	}

	for _, c := range cases {
		result := CompareCards(c.a, c.b, trumpSuit, level, trumpSuit)
		if result != c.expected {
			t.Errorf("CompareCards(%v, %v) = %d, expected %d", c.a, c.b, result, c.expected)
		}
	}
}

func TestCompareNonTrump(t *testing.T) {
	level := Rank10
	trumpSuit := SuitHeart

	spadeA := Card{Suit: SuitSpade, Rank: RankA}
	spadeK := Card{Suit: SuitSpade, Rank: RankK}
	clubA := Card{Suit: SuitClub, Rank: RankA}

	// Same suit: A > K
	if CompareCards(spadeA, spadeK, trumpSuit, level, SuitSpade) != 1 {
		t.Error("Spade A should beat Spade K")
	}
	// Different non-trump suits: lead suit wins
	if CompareCards(spadeA, clubA, trumpSuit, level, SuitSpade) != 1 {
		t.Error("Lead suit A should beat off-suit A")
	}
	// Trump beats non-trump
	trumpA := Card{Suit: SuitHeart, Rank: RankA}
	if CompareCards(trumpA, spadeA, trumpSuit, level, SuitSpade) != 1 {
		t.Error("Trump should beat non-trump")
	}
}

func TestCardPoints(t *testing.T) {
	c5 := Card{Rank: Rank5}
	if c5.Points() != 5 {
		t.Error("5 should be 5 points")
	}
	c10 := Card{Rank: Rank10}
	if c10.Points() != 10 {
		t.Error("10 should be 10 points")
	}
	ck := Card{Rank: RankK}
	if ck.Points() != 10 {
		t.Error("K should be 10 points")
	}
	ca := Card{Rank: RankA}
	if ca.Points() != 0 {
		t.Error("A should be 0 points")
	}
}

func TestValidateLeadingPlay(t *testing.T) {
	level := Rank10
	trumpSuit := SuitHeart

	// Single card is always valid
	single := []Card{{Suit: SuitSpade, Rank: RankA}}
	if !ValidatePlay(single, nil, single, trumpSuit, level) {
		t.Error("Single card lead should be valid")
	}

	// Cards of different suits are invalid
	mixed := []Card{
		{Suit: SuitSpade, Rank: RankA},
		{Suit: SuitClub, Rank: RankK},
	}
	if ValidatePlay(mixed, nil, mixed, trumpSuit, level) {
		t.Error("Mixed suit lead should be invalid")
	}
}

func TestAIPlayNoCrash(t *testing.T) {
	// Simulate a full game with 4 AI players to ensure no panics
	rng := rand.New(rand.NewSource(42))

	g := &Game{
		Level: [2]Rank{Rank3, Rank3},
		rng:   rng,
	}
	g.Players[PositionSouth] = NewPlayer(PositionSouth, false) // All AI
	g.Players[PositionWest] = NewPlayer(PositionWest, false)
	g.Players[PositionNorth] = NewPlayer(PositionNorth, false)
	g.Players[PositionEast] = NewPlayer(PositionEast, false)
	g.Dealer = PlayerPosition(rng.Intn(4))

	// Play a few hands
	for hand := 0; hand < 3; hand++ {
		g.Deal()
		g.RunBiddingPhase()
		if g.TrumpSuit == SuitJoker && g.CurrentBid == nil {
			g.TrumpSuit = SuitHeart // fallback for no-trump games in test
		}
		g.DiscardBottom()

		g.TrickCount = 0
		g.TeamScore = [2]int{0, 0}
		leadPlayer := g.Dealer

		for g.TrickCount < 25 {
			level := g.DealerLevel()
			trick := NewTrick(leadPlayer, g.TrumpSuit, level)
			g.CurrentTrick = trick

			currentPlayer := leadPlayer
			for i := 0; i < 4; i++ {
				player := g.Players[currentPlayer]
				var cards []Card
				if trick.PlayerCount() == 0 {
					cards = aiLead(player, g.TrumpSuit, level)
				} else {
					cards = aiFollow(player, trick, g.TrumpSuit, level, g)
				}

				// Validate
				var leadCards []Card
				if trick.PlayerCount() > 0 {
					leadCards = trick.LeadCards()
				}
				if !ValidatePlay(cards, leadCards, player.Hand, g.TrumpSuit, level) {
					cards = aiSafePlay(player, trick, g.TrumpSuit, level)
				}

				trick.AddPlay(currentPlayer, cards)
				player.RemoveCards(cards)
				currentPlayer = currentPlayer.Next()
			}

			winner := trick.Winner()
			points := trick.Points()
			winnerTeam := PlayerTeam(winner)
			g.TeamScore[winnerTeam] += points
			g.TrickCount++
			leadPlayer = winner
		}

		fmt.Printf("Hand %d: Team0=%d Team1=%d\n", hand, g.TeamScore[0], g.TeamScore[1])
	}
}
