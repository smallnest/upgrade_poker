package main

// Game rules: trump determination, card comparison, tractor detection, play validation

// IsTrump checks if a card is a trump card
// Trump cards: Jokers, level-rank cards of any suit, all 2s (常主), cards of trump suit
func IsTrump(card Card, trumpSuit Suit, level Rank) bool {
	// Jokers are always trump
	if card.IsJoker() {
		return true
	}
	// Level rank cards are always trump (e.g., all 10s when playing level 10)
	if card.Rank == level {
		return true
	}
	// 2 is 常主, always trump
	if card.Rank == Rank2 {
		return true
	}
	// Cards of the trump suit are trump
	if card.Suit == trumpSuit {
		return true
	}
	return false
}

// TrumpOrder returns the sort order of a trump card (higher = stronger)
// Order: BigJoker(100) > SmallJoker(90) > MainLevel(80) > OffLevel(70+) > Main2(60) > Off2(50+) > TrumpA(40) > ... > Trump3(1)
func TrumpOrder(card Card, trumpSuit Suit, level Rank) int {
	if !IsTrump(card, trumpSuit, level) {
		return 0
	}

	// Jokers
	if card.IsJoker() {
		if card.Rank == RankBigJoker {
			return 100
		}
		return 90
	}

	// Level rank cards
	if card.Rank == level {
		if card.Suit == trumpSuit {
			return 80 // Main level rank
		}
		// Off-suit level rank: ordered by suit
		return 70 + int(card.Suit)
	}

	// 2s (常主)
	if card.Rank == Rank2 {
		if card.Suit == trumpSuit {
			return 60 // Main 2
		}
		// Off-suit 2s
		return 50 + int(card.Suit)
	}

	// Trump suit cards (not level rank, not 2)
	if card.Suit == trumpSuit {
		return int(card.Rank) // 3=3, 4=4, ..., A=14
	}

	return 0 // shouldn't reach here
}

// CompareCards compares two cards in the context of a trick
// Returns: -1 if a < b, 0 if equal, 1 if a > b
// This considers trump status and the lead suit
func CompareCards(a, b Card, trumpSuit Suit, level Rank, leadSuit Suit) int {
	aIsTrump := IsTrump(a, trumpSuit, level)
	bIsTrump := IsTrump(b, trumpSuit, level)

	// Trump always beats non-trump
	if aIsTrump && !bIsTrump {
		return 1
	}
	if !aIsTrump && bIsTrump {
		return -1
	}

	// Both trump: compare by trump order
	if aIsTrump && bIsTrump {
		ao := TrumpOrder(a, trumpSuit, level)
		bo := TrumpOrder(b, trumpSuit, level)
		if ao > bo {
			return 1
		}
		if ao < bo {
			return -1
		}
		// Same order (e.g., two off-suit level ranks of different suits) - same strength
		return 0
	}

	// Both non-trump
	// Must be same suit to compare
	aEffectiveSuit := EffectiveSuit(a, trumpSuit, level)
	bEffectiveSuit := EffectiveSuit(b, trumpSuit, level)

	// Cards of non-lead suit can't beat lead suit
	if aEffectiveSuit != bEffectiveSuit {
		if aEffectiveSuit == leadSuit {
			return 1
		}
		if bEffectiveSuit == leadSuit {
			return -1
		}
		// Both off-suit: can't compare, equal (neither wins)
		return 0
	}

	// Same suit, compare rank (higher rank wins)
	if a.Rank > b.Rank {
		return 1
	}
	if a.Rank < b.Rank {
		return -1
	}
	return 0
}

// EffectiveSuit returns the effective suit of a card for trick purposes
// Trump cards effectively belong to the "trump" suit
func EffectiveSuit(card Card, trumpSuit Suit, level Rank) Suit {
	if IsTrump(card, trumpSuit, level) {
		return trumpSuit
	}
	return card.Suit
}

// CardGroup represents a group of cards (single, pair, or tractor)
type CardGroup struct {
	Cards    []Card
	IsTractor bool
	IsPair    bool
	IsSingle  bool
	Suit     Suit // effective suit
}

// AnalyzePlay breaks down a player's played cards into groups
// (singles, pairs, tractors) for comparison with the lead
func AnalyzePlay(cards []Card, trumpSuit Suit, level Rank) []CardGroup {
	if len(cards) == 0 {
		return nil
	}

	// Group by effective suit
	suitGroups := make(map[Suit][]Card)
	for _, c := range cards {
		s := EffectiveSuit(c, trumpSuit, level)
		suitGroups[s] = append(suitGroups[s], c)
	}

	var result []CardGroup
	for suit, group := range suitGroups {
		groups := analyzeSameSuit(group, trumpSuit, level)
		for _, g := range groups {
			g.Suit = suit
			result = append(result, g)
		}
	}

	return result
}

// analyzeSameSuit analyzes cards of the same effective suit into groups
func analyzeSameSuit(cards []Card, trumpSuit Suit, level Rank) []CardGroup {
	if len(cards) == 0 {
		return nil
	}

	// Count cards by rank
	rankCount := make(map[Rank]int)
	for _, c := range cards {
		rankCount[c.Rank]++
	}

	// Find pairs (ranks with count >= 2)
	pairs := make(map[Rank]bool)
	for r, count := range rankCount {
		if count >= 2 {
			pairs[r] = true
		}
	}

	// Find tractors (consecutive pairs)
	var pairRanks []Rank
	for r := range pairs {
		pairRanks = append(pairRanks, r)
	}
	// Sort pair ranks
	sortRanks(pairRanks, trumpSuit, level)

	// Find tractors (consecutive pairs)
	tractors := findTractors(pairRanks, trumpSuit, level)

	// Track which ranks are consumed by tractors
	consumedByTractor := make(map[Rank]bool)
	for _, tRanks := range tractors {
		for _, r := range tRanks {
			consumedByTractor[r] = true
		}
	}

	var result []CardGroup

	// Add tractors
	for _, tRanks := range tractors {
		var tc []Card
		for _, r := range tRanks {
			tc = append(tc, getCardsOfRank(cards, r, 2)...)
		}
		result = append(result, CardGroup{
			Cards:     tc,
			IsTractor: true,
		})
	}

	// Add remaining pairs (not in tractors)
	for r := range pairs {
		if !consumedByTractor[r] {
			result = append(result, CardGroup{
				Cards:  getCardsOfRank(cards, r, 2),
				IsPair: true,
			})
		}
	}

	// Add singles
	usedCards := make(map[Card]bool)
	for _, g := range result {
		for _, c := range g.Cards {
			usedCards[c] = true
		}
	}
	for _, c := range cards {
		if !usedCards[c] {
			result = append(result, CardGroup{
				Cards:   []Card{c},
				IsSingle: true,
			})
		}
	}

	return result
}

// findTractors finds consecutive pairs that form tractors
func findTractors(pairRanks []Rank, trumpSuit Suit, level Rank) [][]Rank {
	if len(pairRanks) < 2 {
		return nil
	}

	// Sort by effective rank order
	sortRanks(pairRanks, trumpSuit, level)

	var tractors [][]Rank
	current := []Rank{pairRanks[0]}

	for i := 1; i < len(pairRanks); i++ {
		if areConsecutiveRanks(current[len(current)-1], pairRanks[i], trumpSuit, level) {
			current = append(current, pairRanks[i])
		} else {
			if len(current) >= 2 {
				tractors = append(tractors, append([]Rank{}, current...))
			}
			current = []Rank{pairRanks[i]}
		}
	}
	if len(current) >= 2 {
		tractors = append(tractors, append([]Rank{}, current...))
	}

	return tractors
}

// areConsecutiveRanks checks if two ranks are consecutive in trump order or normal order
func areConsecutiveRanks(a, b Rank, trumpSuit Suit, level Rank) bool {
	// For trump ranks, use trump order
	// For non-trump ranks, use normal rank order
	// This is simplified: we check if they differ by 1 in the relevant ordering

	// For level rank cards and 2s in trump, the ordering is:
	// MainLevel > OffLevel > Main2 > Off2 > A > K > Q > J > 9 > 8 > 7 > 6 > 5 > 4 > 3
	// We need to check if they're adjacent in this ordering

	aOrder := trumpRankOrder(a, trumpSuit, level)
	bOrder := trumpRankOrder(b, trumpSuit, level)

	diff := aOrder - bOrder
	if diff < 0 {
		diff = -diff
	}
	return diff == 1
}

// trumpRankOrder gives a linear order for trump rank adjacency
func trumpRankOrder(r Rank, trumpSuit Suit, level Rank) int {
	if r == level {
		return 20 // Level rank (highest in trump after jokers)
	}
	if r == Rank2 {
		return 10 // 2 is 常主, after level rank
	}
	// Normal rank order
	return int(r)
}

// sortRanks sorts ranks by their trump order (descending)
func sortRanks(ranks []Rank, trumpSuit Suit, level Rank) {
	sortSlice(ranks, func(a, b Rank) bool {
		return trumpRankOrder(a, trumpSuit, level) > trumpRankOrder(b, trumpSuit, level)
	})
}

func sortSlice(ranks []Rank, less func(a, b Rank) bool) {
	for i := 0; i < len(ranks); i++ {
		for j := i + 1; j < len(ranks); j++ {
			if less(ranks[j], ranks[i]) {
				ranks[i], ranks[j] = ranks[j], ranks[i]
			}
		}
	}
}

// getCardsOfRank returns up to n cards of the given rank from the slice
func getCardsOfRank(cards []Card, rank Rank, n int) []Card {
	var result []Card
	for _, c := range cards {
		if c.Rank == rank && len(result) < n {
			result = append(result, c)
		}
	}
	return result
}

// ValidatePlay checks if the played cards are a legal play given the lead and the player's hand
func ValidatePlay(played []Card, lead []Card, hand []Card, trumpSuit Suit, level Rank) bool {
	if len(played) == 0 {
		return false
	}

	// If leading (no lead cards), any valid combination is OK
	if len(lead) == 0 {
		return validateLeading(played, trumpSuit, level)
	}

	// Following: must follow suit and match the structure of the lead
	return validateFollowing(played, lead, hand, trumpSuit, level)
}

// validateLeading checks if a leading play is valid (甩牌 rules)
func validateLeading(cards []Card, trumpSuit Suit, level Rank) bool {
	// Leading play: all cards must be of the same effective suit
	// (or all trump), and form valid groups (singles, pairs, tractors)

	if len(cards) == 0 {
		return false
	}

	// All cards must be of the same effective suit
	firstSuit := EffectiveSuit(cards[0], trumpSuit, level)
	for _, c := range cards[1:] {
		if EffectiveSuit(c, trumpSuit, level) != firstSuit {
			return false
		}
	}

	// The play must decompose cleanly into singles, pairs, and tractors
	groups := AnalyzePlay(cards, trumpSuit, level)
	totalCards := 0
	for _, g := range groups {
		totalCards += len(g.Cards)
	}
	return totalCards == len(cards)
}

// validateFollowing checks if a following play is valid
func validateFollowing(played []Card, lead []Card, hand []Card, trumpSuit Suit, level Rank) bool {
	leadSuit := EffectiveSuit(lead[0], trumpSuit, level)

	// Must play the same number of cards as the lead
	if len(played) != len(lead) {
		return false
	}

	// Count lead-suit cards in hand
	leadSuitInHand := 0
	for _, c := range hand {
		if EffectiveSuit(c, trumpSuit, level) == leadSuit {
			leadSuitInHand++
		}
	}

	// Count lead-suit cards in the played set
	leadSuitPlayed := 0
	for _, c := range played {
		if EffectiveSuit(c, trumpSuit, level) == leadSuit {
			leadSuitPlayed++
		}
	}

	// Must use all lead-suit cards from hand if we have fewer than needed
	if leadSuitInHand > 0 {
		if leadSuitPlayed < min(leadSuitInHand, len(lead)) {
			return false
		}
	}

	leadGroups := AnalyzePlay(lead, trumpSuit, level)
	playedGroups := AnalyzePlay(played, trumpSuit, level)

	// Count tractors and pairs in lead
	leadTractorCount := 0
	leadPairCount := 0
	for _, g := range leadGroups {
		if g.IsTractor {
			leadTractorCount++
			leadPairCount += len(g.Cards) / 2
		} else if g.IsPair {
			leadPairCount++
		}
	}

	playedTractorCount := 0
	playedPairCount := 0
	for _, g := range playedGroups {
		if g.IsTractor {
			playedTractorCount++
			playedPairCount += len(g.Cards) / 2
		} else if g.IsPair {
			playedPairCount++
		}
	}

	if leadSuitInHand > 0 {
		// Must match structure: if lead has pairs, must play pairs if available
		if leadPairCount > 0 {
			availablePairs := countAvailablePairs(hand, leadSuit, trumpSuit, level)
			if availablePairs > 0 && playedPairCount < min(availablePairs, leadPairCount) {
				return false
			}
		}

		// Must match tractor count if possible
		if leadTractorCount > 0 {
			availableTractors := countAvailableTractors(hand, leadSuit, trumpSuit, level)
			if availableTractors > 0 && playedTractorCount < min(availableTractors, leadTractorCount) {
				return false
			}
		}

		return true
	}

	// No lead suit cards: can play any cards
	// But if playing trump (毙牌), must meet minimum tractor and pair counts
	allTrump := true
	for _, c := range played {
		if !IsTrump(c, trumpSuit, level) {
			allTrump = false
			break
		}
	}

	if allTrump {
		// 毙牌: tractor and pair counts must meet or exceed lead's
		if playedTractorCount < leadTractorCount {
			return false
		}
		if playedPairCount < leadPairCount {
			return false
		}
	}

	return true
}

// countAvailablePairs counts pairs available in hand for a given suit
func countAvailablePairs(hand []Card, suit Suit, trumpSuit Suit, level Rank) int {
	rankCount := make(map[Rank]int)
	for _, c := range hand {
		if EffectiveSuit(c, trumpSuit, level) == suit {
			rankCount[c.Rank]++
		}
	}
	count := 0
	for _, c := range rankCount {
		if c >= 2 {
			count++
		}
	}
	return count
}

// countAvailableTractors counts tractors available in hand for a given suit
func countAvailableTractors(hand []Card, suit Suit, trumpSuit Suit, level Rank) int {
	rankCount := make(map[Rank]int)
	for _, c := range hand {
		if EffectiveSuit(c, trumpSuit, level) == suit {
			rankCount[c.Rank]++
		}
	}

	var pairRanks []Rank
	for r, c := range rankCount {
		if c >= 2 {
			pairRanks = append(pairRanks, r)
		}
	}

	if len(pairRanks) < 2 {
		return 0
	}

	tractors := findTractors(pairRanks, trumpSuit, level)
	return len(tractors)
}

// IsKillPlay determines if the played cards constitute a "毙" (kill with trump)
func IsKillPlay(played []Card, lead []Card, trumpSuit Suit, level Rank) bool {
	// All played cards must be trump
	for _, c := range played {
		if !IsTrump(c, trumpSuit, level) {
			return false
		}
	}

	// Lead must be non-trump (if lead is also trump, it's not a "kill")
	for _, c := range lead {
		if !IsTrump(c, trumpSuit, level) {
			return true // At least one non-trump card in lead, and all played are trump
		}
	}

	return false
}

// DetermineTrickWinner determines which player wins the trick
// Returns the index of the winning player
func DetermineTrickWinner(plays [][]Card, trumpSuit Suit, level Rank) int {
	if len(plays) == 0 {
		return 0
	}

	// Find first non-empty play to determine lead suit
	leadIdx := -1
	for i, p := range plays {
		if len(p) > 0 {
			leadIdx = i
			break
		}
	}

	if leadIdx == -1 {
		return 0 // All plays empty, first player wins by default
	}

	leadSuit := EffectiveSuit(plays[leadIdx][0], trumpSuit, level)
	winner := leadIdx

	for i := 0; i < len(plays); i++ {
		if i == leadIdx {
			continue
		}
		if len(plays[i]) == 0 {
			continue
		}
		cmp := comparePlays(plays[i], plays[winner], trumpSuit, level, leadSuit)
		if cmp > 0 {
			winner = i
		}
		// cmp == 0: first player wins (先出者大), keep current winner
	}

	return winner
}

// comparePlays compares two plays to determine which is stronger
// Returns: 1 if a > b, -1 if a < b, 0 if equal
func comparePlays(a, b []Card, trumpSuit Suit, level Rank, leadSuit Suit) int {
	aBest := bestCardInPlay(a, trumpSuit, level, leadSuit)
	bBest := bestCardInPlay(b, trumpSuit, level, leadSuit)

	// If either is empty, the non-empty one wins
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return -1
	}
	if len(b) == 0 {
		return 1
	}

	return CompareCards(aBest, bBest, trumpSuit, level, leadSuit)
}

// bestCardInPlay returns the strongest card in a play
func bestCardInPlay(cards []Card, trumpSuit Suit, level Rank, leadSuit Suit) Card {
	if len(cards) == 0 {
		return Card{}
	}

	best := cards[0]
	for _, c := range cards[1:] {
		if CompareCards(c, best, trumpSuit, level, leadSuit) > 0 {
			best = c
		}
	}
	return best
}

// CalculateTrickPoints calculates the total points in a trick
func CalculateTrickPoints(plays [][]Card) int {
	total := 0
	for _, play := range plays {
		for _, card := range play {
			total += card.Points()
		}
	}
	return total
}

// CalculateBottomMultiplier calculates the bottom card score multiplier
// based on the last trick's winning play structure
// Multiplier = 2^(number of cards in the largest group type)
func CalculateBottomMultiplier(winningPlay []Card, trumpSuit Suit, level Rank) int {
	if len(winningPlay) == 0 {
		return 2 // single card = 2x
	}

	groups := AnalyzePlay(winningPlay, trumpSuit, level)
	maxGroupSize := 1
	for _, g := range groups {
		if len(g.Cards) > maxGroupSize {
			maxGroupSize = len(g.Cards)
		}
	}

	// Multiplier = 2^maxGroupSize
	result := 1
	for i := 0; i < maxGroupSize; i++ {
		result *= 2
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
