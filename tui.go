package main

import (
	"fmt"
	"math/rand"
	"time"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// UIPhase represents the current UI interaction phase
type UIPhase int

const (
	UIPhaseWelcome   UIPhase = iota
	UIPhaseDealing
	UIPhaseBidding
	UIPhaseDiscard
	UIPhasePlaying
	UIPhaseWaitTrick  // waiting for trick result display
	UIPhaseHandResult // show hand result
	UIPhaseGameOver
)

// UserAction represents an action from the user
type UserAction struct {
	Type     string // "play", "cancel", "bid", "pass", "confirm", "start"
	CardIdx  []int  // selected card indices (for play/discard)
	BidType  BidType
	BidSuit  Suit
}

// CardRect represents a clickable card area on screen
type CardRect struct {
	Index int // index in the display order
	X, Y  int // top-left position
	W, H  int // width and height
}

// Button represents a clickable button
type Button struct {
	Label  string
	Action string
	X, Y   int
	W      int
}

// TUI manages the terminal user interface
type TUI struct {
	screen     tcell.Screen
	game       *Game
	phase      UIPhase
	selected   map[int]bool  // selected card indices
	cardRects  []CardRect    // card click areas
	buttons    []Button      // button click areas
	actionChan chan UserAction
	width      int
	height     int
	rng        *rand.Rand

	// Messages to display
	message    string
	msgButtons []Button

	// Discard phase: how many cards to discard
	discardCount int

	// Keyboard cursor for card navigation
	cursorIdx int

	// Bidding options
	bidOptions []Bid

	// Trick display timer
	trickWinner PlayerPosition
	trickPoints int

	// Cross-goroutine redraw
	redishReq  chan struct{}
	redishDone chan struct{}

	// AI thinking animation
	thinkingPos PlayerPosition
	thinking    bool

		// Quit flag
		quitting bool

		// Dealing animation
		dealCounts [4]int

		// Waiting for human to play
		waitingForHuman bool

		lastEscTime time.Time

		// Double-click detection
		lastClickTime time.Time
		lastClickX    int
		lastClickY    int
}

func NewTUI(g *Game) *TUI {
	return &TUI{
		game:       g,
		phase:      UIPhaseWelcome,
		selected:   make(map[int]bool),
		actionChan: make(chan UserAction, 10),
		redishReq:  make(chan struct{}),
		redishDone: make(chan struct{}, 1),
		rng:        rand.New(rand.NewSource(rand.Int63())),
	}
}

// Init initializes the TUI screen
func (t *TUI) Init() error {
	s, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("creating screen: %w", err)
	}
	if err := s.Init(); err != nil {
		return fmt.Errorf("initializing screen: %w", err)
	}
	s.EnableMouse()
	s.EnablePaste()
	t.screen = s
	t.width, t.height = s.Size()
	return nil
}

// Close shuts down the TUI
func (t *TUI) Close() {
	if t.screen != nil {
		t.screen.Fini()
	}
}

// Run starts the TUI event loop (runs in main goroutine)
func (t *TUI) Run() {
	// Start game logic in separate goroutine
	go t.game.RunTUI(t)

	// Redraw notifier: game goroutine sends on redishReq, we inject an event
	go func() {
		for range t.redishReq {
			t.screen.PostEvent(tcell.NewEventInterrupt(nil))
			// Wait for the main loop to finish redraw before sending next
			<-t.redishDone
		}
	}()

	for {
		t.draw()
		t.screen.Show()

		ev := t.screen.PollEvent()
		if ev == nil {
			continue
		}

		switch ev := ev.(type) {
		case *tcell.EventResize:
			t.width, t.height = ev.Size()
			t.screen.Sync()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlC {
				t.quitting = true
				t.actionChan <- UserAction{Type: "quit"}
				t.screen.Fini()
				return
			}
			if ev.Key() == tcell.KeyEsc {
				if !t.lastEscTime.IsZero() && time.Since(t.lastEscTime) < 2*time.Second {
					t.quitting = true
					t.actionChan <- UserAction{Type: "quit"}
					t.screen.Fini()
					return
				}
				t.lastEscTime = time.Now()
			} else {
				t.handleKey(ev)
			}
		case *tcell.EventMouse:
			t.handleMouse(ev)
		case *tcell.EventInterrupt:
			// Redraw triggered by game goroutine
			t.draw()
			t.screen.Show()
			t.redishDone <- struct{}{}
		}
	}
}

// handleKey processes keyboard events
func (t *TUI) handleKey(ev *tcell.EventKey) {
	switch t.phase {
	case UIPhaseWelcome:
		t.actionChan <- UserAction{Type: "start"}
	case UIPhasePlaying:
		t.handleCardNav(ev)
		switch ev.Key() {
		case tcell.KeyEnter:
			t.submitSelection()
		case tcell.KeyBackspace, tcell.KeyDelete:
			t.selected = make(map[int]bool)
	t.cursorIdx = 0
		default:
			switch ev.Rune() {
			case 'p', 'P': // 出牌
				t.submitSelection()
			case 'c', 'C': // 取消
				t.selected = make(map[int]bool)
	t.cursorIdx = 0
			}
		}
	case UIPhaseDiscard:
		t.handleCardNav(ev)
		switch ev.Key() {
		case tcell.KeyEnter:
			t.submitSelection()
		case tcell.KeyBackspace, tcell.KeyDelete:
			t.selected = make(map[int]bool)
	t.cursorIdx = 0
		default:
			switch ev.Rune() {
			case 'd', 'D': // 扣底
				t.submitSelection()
			case 'c', 'C': // 取消
				t.selected = make(map[int]bool)
	t.cursorIdx = 0
			}
		}
	case UIPhaseHandResult:
		t.actionChan <- UserAction{Type: "confirm"}
	case UIPhaseBidding:
		switch ev.Rune() {
		case 'b', 'B': // 亮主
			if len(t.bidOptions) > 0 {
				bid := t.bidOptions[0]
				t.actionChan <- UserAction{Type: "bid", BidType: bid.Type, BidSuit: bid.Suit}
			}
		case 'p', 'P': // 不亮
			t.actionChan <- UserAction{Type: "pass"}
		}
	case UIPhaseGameOver:
		if ev.Key() == tcell.KeyEnter {
			t.screen.Fini()
			return
		}
	}
}

// handleCardNav handles arrow key and space navigation for card selection
func (t *TUI) handleCardNav(ev *tcell.EventKey) {
	totalCards := len(t.cardRects)
	if totalCards == 0 {
		return
	}

	// Clamp cursor
	if t.cursorIdx >= totalCards {
		t.cursorIdx = totalCards - 1
	}
	if t.cursorIdx < 0 {
		t.cursorIdx = 0
	}

	switch ev.Key() {
	case tcell.KeyLeft:
		if t.cursorIdx > 0 {
			t.cursorIdx--
		}
	case tcell.KeyRight:
		if t.cursorIdx < totalCards-1 {
			t.cursorIdx++
		}
	case tcell.KeyUp:
		// Move up one row (approximate: cardsPerRow)
		cardsPerRow := (t.width - 4) / 6
		if cardsPerRow < 1 {
			cardsPerRow = 1
		}
		if t.cursorIdx-cardsPerRow >= 0 {
			t.cursorIdx -= cardsPerRow
		}
	case tcell.KeyDown:
		cardsPerRow := (t.width - 4) / 6
		if cardsPerRow < 1 {
			cardsPerRow = 1
		}
		if t.cursorIdx+cardsPerRow < totalCards {
			t.cursorIdx += cardsPerRow
		}
	default:
		if ev.Rune() == ' ' {
			// Space toggles selection on cursor card
			if t.selected[t.cursorIdx] {
				delete(t.selected, t.cursorIdx)
			} else {
				t.selected[t.cursorIdx] = true
			}
		}
	}
}

// SleepForRedraw sleeps for the given duration while periodically redrawing the screen.
// Called from the game goroutine to keep UI responsive during pauses.
func (t *TUI) SleepForRedraw(d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if t.quitting {
			return
		}
		time.Sleep(80 * time.Millisecond)
		select {
		case t.redishReq <- struct{}{}:
		default:
		}
	}
}

// handleMouse processes mouse events
func (t *TUI) handleMouse(ev *tcell.EventMouse) {
	x, y := ev.Position()
	btn := ev.Buttons()

	if btn&tcell.Button1 != 0 {
		// Detect double-click: same position within 500ms
		now := time.Now()
		isDoubleClick := false
		if !t.lastClickTime.IsZero() && now.Sub(t.lastClickTime) < 500*time.Millisecond &&
			t.lastClickX == x && t.lastClickY == y {
			isDoubleClick = true
		}
		t.lastClickTime = now
		t.lastClickX = x
		t.lastClickY = y

		// Check card clicks
		for _, cr := range t.cardRects {
			if x >= cr.X && x < cr.X+cr.W && y >= cr.Y && y < cr.Y+cr.H {
				if t.phase == UIPhasePlaying {
					if isDoubleClick {
						// Double-click: directly play this card
						t.selected = map[int]bool{cr.Index: true}
						t.submitSelection()
					} else {
						t.cursorIdx = cr.Index
						if t.selected[cr.Index] {
							delete(t.selected, cr.Index)
						} else {
							t.selected[cr.Index] = true
						}
					}
				} else if t.phase == UIPhaseDiscard {
					t.cursorIdx = cr.Index
					if t.selected[cr.Index] {
						delete(t.selected, cr.Index)
					} else {
						t.selected[cr.Index] = true
					}
				}
				return
			}
		}

		// Check button clicks
		for _, b := range t.buttons {
			if y == b.Y && x >= b.X && x < b.X+b.W {
				t.handleButton(b.Action)
				return
			}
		}

		// Check message buttons
		for _, b := range t.msgButtons {
			if y == b.Y && x >= b.X && x < b.X+b.W {
				t.handleButton(b.Action)
				return
			}
		}
	}
}

// handleButton processes button clicks
func (t *TUI) handleButton(action string) {
	switch action {
	case "play":
		t.submitSelection()
	case "cancel":
		t.selected = make(map[int]bool)
	t.cursorIdx = 0
	case "bid":
		// Find the best bid option
		if len(t.bidOptions) > 0 {
			bid := t.bidOptions[0]
			t.actionChan <- UserAction{Type: "bid", BidType: bid.Type, BidSuit: bid.Suit}
		}
	case "pass":
		t.actionChan <- UserAction{Type: "pass"}
	case "confirm":
		t.actionChan <- UserAction{Type: "confirm"}
	case "start":
		t.actionChan <- UserAction{Type: "start"}
	}
}

// submitSelection submits the current card selection
func (t *TUI) submitSelection() {
	if len(t.selected) == 0 {
		return
	}

	if t.phase == UIPhasePlaying {
		indices := make([]int, 0, len(t.selected))
		for idx := range t.selected {
			indices = append(indices, idx)
		}
		t.actionChan <- UserAction{Type: "play", CardIdx: indices}
	} else if t.phase == UIPhaseDiscard {
		if len(t.selected) != t.discardCount {
			return
		}
		indices := make([]int, 0, len(t.selected))
		for idx := range t.selected {
			indices = append(indices, idx)
		}
		t.actionChan <- UserAction{Type: "play", CardIdx: indices}
	}
}

// WaitForAction blocks until the user performs an action
func (t *TUI) WaitForAction() UserAction {
	action := <-t.actionChan
	if action.Type == "quit" {
		t.screen.Fini()
		os.Exit(0)
	}
	return action
}

// WaitForActionOrTimeout waits for user action or timeout, whichever comes first.
// Returns the action and whether it was a timeout.
func (t *TUI) WaitForActionOrTimeout(timeout time.Duration) (UserAction, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case action := <-t.actionChan:
		if action.Type == "quit" {
			t.screen.Fini()
			os.Exit(0)
		}
		return action, false
	case <-timer.C:
		return UserAction{Type: "timeout"}, true
	}
}

// SetPhase changes the UI phase and resets selection
func (t *TUI) SetPhase(phase UIPhase) {
	t.phase = phase
	t.selected = make(map[int]bool)
	t.cursorIdx = 0
	t.cardRects = nil
	t.buttons = nil
	t.message = ""
	t.msgButtons = nil
}

// SetMessage displays a message with optional buttons
func (t *TUI) SetMessage(msg string, buttons []Button) {
	t.message = msg
	t.msgButtons = buttons
}

// ============ Drawing Functions ============

func (t *TUI) draw() {
	t.screen.Clear()
	t.cardRects = nil
	t.buttons = nil

	t.drawStatus()
	t.drawNorthPlayer()
	t.drawWestPlayer()
	t.drawEastPlayer()
	t.drawTrickArea()
	t.drawSouthHand()
	t.drawActionButtons()
	t.drawMessage()

	t.screen.Show()
}

// drawStatus draws the top status bar
func (t *TUI) drawStatus() {
	g := t.game
	level := g.DealerLevel()
	opponentTeam := g.OpponentTeam()
	opponentLevel := g.Level[opponentTeam]

	var trumpStr string
	if g.TrumpSuit == SuitJoker {
		trumpStr = "无主"
	} else {
		trumpStr = g.TrumpSuit.Symbol() + "主"
	}

	// Background bar
	bgStyle := tcell.StyleDefault.Reverse(true)
	for x := 0; x < t.width; x++ {
		t.screen.SetContent(x, 0, ' ', nil, bgStyle)
	}

	// Highlight style for values
	valStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Reverse(true).Bold(true)

	// Left side: title + dealer + levels
	t.drawStringW(" 升级(拖拉机)  庄家:", 1, 0, bgStyle)
	x := 1 + runewidth.StringWidth(" 升级(拖拉机)  庄家:")
	t.drawStringW(formatPosition(g.Dealer), x, 0, valStyle)
	x += runewidth.StringWidth(formatPosition(g.Dealer))
	t.drawStringW(" 庄打", x, 0, bgStyle)
	x += runewidth.StringWidth(" 庄打")
	t.drawStringW(LevelDisplayName(level), x, 0, valStyle)
	x += runewidth.StringWidth(LevelDisplayName(level))
	t.drawStringW(" 闲打", x, 0, bgStyle)
	x += runewidth.StringWidth(" 闲打")
	t.drawStringW(LevelDisplayName(opponentLevel), x, 0, valStyle)

	// Right side: trump, score
	opponentName := formatTeam(opponentTeam)
	scoreStr := fmt.Sprintf("%d分", g.TeamScore[opponentTeam])

	rightParts := []struct {
		text  string
		style tcell.Style
	}{
		{trumpStr, valStyle},
		{" " + opponentName + "(闲):", bgStyle},
		{scoreStr, valStyle},
	}

	// Calculate total display width
	totalW := 0
	for _, p := range rightParts {
		totalW += runewidth.StringWidth(p.text)
	}

	// Draw right-aligned
	rx := t.width - totalW - 1
	for _, p := range rightParts {
		t.drawStringW(p.text, rx, 0, p.style)
		rx += runewidth.StringWidth(p.text)
	}
}

// drawStringW draws a string respecting display width for positioning
func (t *TUI) drawStringW(s string, x, y int, style tcell.Style) {
	for _, ch := range s {
		t.screen.SetContent(x, y, ch, nil, style)
		w := runewidth.RuneWidth(ch)
		if w > 1 {
			// fill the extra cells
			for i := 1; i < w; i++ {
				t.screen.SetContent(x+i, y, ' ', nil, style)
			}
		}
		x += w
	}
}

// drawNorthPlayer draws the north AI player
func (t *TUI) drawNorthPlayer() {
	g := t.game
	cx := t.width / 2

	// Player label
	label := "北(AI)"
	style := tcell.StyleDefault
	if g.Dealer == PositionNorth {
		style = style.Bold(true)
	}
	t.drawString(cx-len(label)/2, 2, label, style)


		// Dealing animation: show card backs
		if t.phase == UIPhaseDealing {
			count := t.dealCounts[PositionNorth]
			maxShow := 7
			if count > maxShow {
				count = maxShow
			}
			startX := cx - count*3
			for k := 0; k < count; k++ {
				t.drawCardBack(startX+k*6, 4)
			}
			return
		}
		// Thinking animation
		if t.thinking && t.thinkingPos == PositionNorth {
			t.drawString(cx-6, 4, "思考中...", tcell.StyleDefault.Foreground(tcell.ColorYellow))
			return
		}

	// If this player played cards in current trick, show them
	if g.CurrentTrick != nil {
		if cards, ok := g.CurrentTrick.Plays[PositionNorth]; ok && len(cards) > 0 {
			t.drawCardsRow(cx-len(cards)*4/2, 4, cards, false)
		}
	}
}

// drawWestPlayer draws the west AI player
func (t *TUI) drawWestPlayer() {
	g := t.game
	x := 2

	// Player label
	label := "西(AI)"
	style := tcell.StyleDefault
	if g.Dealer == PositionWest {
		style = style.Bold(true)
	}
	t.drawString(x, t.height/2-2, label, style)

		// Dealing animation: show card backs
		if t.phase == UIPhaseDealing {
			count := t.dealCounts[PositionWest]
			maxShow := 5
			if count > maxShow {
				count = maxShow
			}
			for k := 0; k < count; k++ {
				t.drawCardBack(x, t.height/2-k)
			}
			return
		}
	// Thinking animation
	if t.thinking && t.thinkingPos == PositionWest {
		t.drawString(x, t.height/2, "思考中...", tcell.StyleDefault.Foreground(tcell.ColorYellow))
		return
	}

	// Played cards
	if g.CurrentTrick != nil {
		if cards, ok := g.CurrentTrick.Plays[PositionWest]; ok && len(cards) > 0 {
			t.drawCardsRow(x, t.height/2, cards, false)
		}
	}
}

// drawEastPlayer draws the east AI player
func (t *TUI) drawEastPlayer() {
	g := t.game
	x := t.width - 20

	// Player label
	label := "东(AI)"
	style := tcell.StyleDefault
	if g.Dealer == PositionEast {
		style = style.Bold(true)
	}
	t.drawString(x, t.height/2-2, label, style)

		// Dealing animation: show card backs
		if t.phase == UIPhaseDealing {
			count := t.dealCounts[PositionEast]
			maxShow := 5
			if count > maxShow {
				count = maxShow
			}
			for k := 0; k < count; k++ {
				t.drawCardBack(x, t.height/2-k)
			}
			return
		}
	// Thinking animation
	if t.thinking && t.thinkingPos == PositionEast {
		t.drawString(x, t.height/2, "思考中...", tcell.StyleDefault.Foreground(tcell.ColorYellow))
		return
	}

	// Played cards
	if g.CurrentTrick != nil {
		if cards, ok := g.CurrentTrick.Plays[PositionEast]; ok && len(cards) > 0 {
			t.drawCardsRow(x, t.height/2, cards, false)
		}
	}
}

// drawTrickArea draws the center trick area
func (t *TUI) drawTrickArea() {
	g := t.game
	if g.CurrentTrick == nil {
		return
	}

	// South player's cards in trick (drawn above the hand area)
	if cards, ok := g.CurrentTrick.Plays[PositionSouth]; ok && len(cards) > 0 {
		cx := t.width / 2
		t.drawCardsRow(cx-len(cards)*4/2, t.height-12, cards, false)
	}

		// Show winner info in double-line box in center
		if t.phase == UIPhaseWaitTrick {
			winnerText := fmt.Sprintf("%s 赢得此轮", formatPosition(t.trickWinner))
			pointsText := fmt.Sprintf("获得 %d 分", t.trickPoints)
			cx := t.width / 2
			cy := t.height / 2

			ww := runewidth.StringWidth(winnerText)
			pw := runewidth.StringWidth(pointsText)
			maxW := ww
			if pw > maxW {
				maxW = pw
			}
			boxW := maxW + 8
			boxH := 5
			boxX := cx - boxW/2
			boxY := cy - boxH/2

			bg := tcell.NewRGBColor(20, 40, 20)
			bgStyle := tcell.StyleDefault.Background(bg)
			for by := boxY; by < boxY+boxH; by++ {
				for bx := boxX; bx < boxX+boxW; bx++ {
					t.screen.SetContent(bx, by, ' ', nil, bgStyle)
				}
			}

			bs := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true).Background(bg)
			t.screen.SetContent(boxX, boxY, '\u2554', nil, bs)
			for bx := boxX + 1; bx < boxX+boxW-1; bx++ {
				t.screen.SetContent(bx, boxY, '\u2550', nil, bs)
			}
			t.screen.SetContent(boxX+boxW-1, boxY, '\u2557', nil, bs)
			for by := boxY + 1; by < boxY+boxH-1; by++ {
				t.screen.SetContent(boxX, by, '\u2551', nil, bs)
				t.screen.SetContent(boxX+boxW-1, by, '\u2551', nil, bs)
			}
			t.screen.SetContent(boxX, boxY+boxH-1, '\u255a', nil, bs)
			for bx := boxX + 1; bx < boxX+boxW-1; bx++ {
				t.screen.SetContent(bx, boxY+boxH-1, '\u2550', nil, bs)
			}
			t.screen.SetContent(boxX+boxW-1, boxY+boxH-1, '\u255d', nil, bs)

			ws := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true).Background(bg)
			ps := tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true).Background(bg)
			t.drawStringW(winnerText, cx-ww/2, cy-1, ws)
			t.drawStringW(pointsText, cx-pw/2, cy+1, ps)
		}
}

// drawSouthHand draws the human player's hand at the bottom
func (t *TUI) drawSouthHand() {
	g := t.game
	player := g.Players[PositionSouth]
	if len(player.Hand) == 0 {
			return
		}

	// Dealing animation: show dealt cards so far
	if t.phase == UIPhaseDealing {
		count := t.dealCounts[PositionSouth]
		if count == 0 {
			return
		}
		cardW := 5
		gap := 1
		handY := t.height - 6
		// Center dealing cards horizontally, ensure all cards fit
		maxFit := (t.width - 4) / (cardW + gap)
		dealCount := count
		if dealCount > maxFit {
			dealCount = maxFit
		}
		dealWidth := dealCount*(cardW+gap) - gap
		dealStartX := (t.width - dealWidth) / 2
		if dealStartX < 2 {
			dealStartX = 2
		}
		label := "南(你)"
		t.drawString(t.width/2-runewidth.StringWidth(label)/2, handY-2, label, tcell.StyleDefault.Bold(true))
		for k := 0; k < count; k++ {
			x := dealStartX + k*(cardW+gap)
			if x+cardW > t.width-2 {
				break
			}
				card := player.Hand[k]
				t.drawCard(x, handY, card, false, false, false)
		}
		return
	}

	level := g.DealerLevel()
	player.SortHand(g.TrumpSuit, level)
	groups := GroupBySuit(player.Hand, g.TrumpSuit, level)

	// Card dimensions: 5 wide x 3 tall (to fit "10")
	cardW := 5
	cardH := 3
	gap := 1
	totalCards := len(player.Hand)

	// How many cards fit per row
	cardsPerRow := (t.width - 4) / (cardW + gap)
	if cardsPerRow < 1 {
		cardsPerRow = 1
	}
	numRows := (totalCards + cardsPerRow - 1) / cardsPerRow
	// Center cards horizontally based on full row width to ensure all cards fit
	fullRowWidth := cardsPerRow*(cardW+gap) - gap
	startX := (t.width - fullRowWidth) / 2
	if startX < 2 {
		startX = 2
	}
	handY := t.height - numRows*(cardH+1) - 3
	if handY < t.height/2+2 {
		handY = t.height / 2 + 2
	}

	// Label
	label := "南(你)"
	t.drawString(t.width/2-runewidth.StringWidth(label)/2, handY-2, label, tcell.StyleDefault.Bold(true))

		// Show waiting indicator when it's human's turn
		if t.waitingForHuman {
			waitText := "等你出牌..."
			t.drawString(t.width/2-runewidth.StringWidth(waitText)/2, handY-4, waitText, tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true))
		}

		// Build draw order: trump first, then other suits
		drawOrder := make([]Card, 0, len(player.Hand))
		// Always put trump group first (works for both normal and no-trump)
		if cards, ok := groups[g.TrumpSuit]; ok {
			drawOrder = append(drawOrder, cards...)
		}
		suitOrder := []Suit{SuitSpade, SuitHeart, SuitDiamond, SuitClub}
		for _, suit := range suitOrder {
			if suit == g.TrumpSuit {
				continue
			}
			if cards, ok := groups[suit]; ok {
				drawOrder = append(drawOrder, cards...)
			}
		}
		// For non-no-trump, pick up any cards grouped under SuitJoker
		if g.TrumpSuit != SuitJoker {
			if cards, ok := groups[SuitJoker]; ok {
				drawOrder = append(drawOrder, cards...)
			}
		}

	// Draw cards
	for idx, c := range drawOrder {
		row := idx / cardsPerRow
		col := idx % cardsPerRow
		x := startX + col*(cardW+gap)
		baseY := handY + row*(cardH+1)
		y := baseY
		if t.selected[idx] {
			y -= 2 // lift selected card up 2 rows for visibility
		}
		isTrump := IsTrump(c, g.TrumpSuit, level)
		isCursor := (idx == t.cursorIdx) && (t.phase == UIPhasePlaying || t.phase == UIPhaseDiscard)
			t.drawCard(x, y, c, t.selected[idx], isTrump, isCursor)
		// Draw selection marker above selected cards
		if t.selected[idx] {
			markerStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
			t.screen.SetContent(x+2, y-1, '\u25bc', nil, markerStyle) // ▼ arrow pointing down at card
		}
		// Click area covers full card extent including lifted position
		t.cardRects = append(t.cardRects, CardRect{Index: idx, X: x, Y: baseY - 2, W: cardW, H: cardH + 2})
	}

	t.game.drawOrder = drawOrder
}

// drawCard draws a single card at (x, y), 5 wide x 3 tall
func (t *TUI) drawCard(x, y int, card Card, selected, isTrump, isCursor bool) {
	borderStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	textStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true)
	if selected {
		borderStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.NewRGBColor(0, 80, 0))
		textStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.NewRGBColor(0, 80, 0)).Bold(true)
	} else if isTrump {
		bg := tcell.NewRGBColor(30, 30, 60)
		borderStyle = borderStyle.Background(bg)
		textStyle = textStyle.Background(bg)
	}
	if isCursor && !selected {
		borderStyle = tcell.StyleDefault.Foreground(tcell.ColorDarkCyan).Bold(true)
		textStyle = tcell.StyleDefault.Foreground(tcell.ColorDarkCyan).Bold(true)
	}

	// Suit symbol: red for hearts/diamonds, white otherwise
	suitRune := suitToRune(card.Suit)
	suitStyle := textStyle
	if card.Suit == SuitHeart || card.Suit == SuitDiamond {
		suitStyle = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
		if selected {
			suitStyle = tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.NewRGBColor(0, 80, 0)).Bold(true)
		} else if isTrump {
			suitStyle = tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.NewRGBColor(30, 30, 60)).Bold(true)
		}
	}

	// Top border
	t.screen.SetContent(x, y, '\u250c', nil, borderStyle)
	t.screen.SetContent(x+1, y, '\u2500', nil, borderStyle)
	t.screen.SetContent(x+2, y, '\u2500', nil, borderStyle)
	t.screen.SetContent(x+3, y, '\u2500', nil, borderStyle)
	t.screen.SetContent(x+4, y, '\u2510', nil, borderStyle)

	// Content line: suit + rank
	t.screen.SetContent(x, y+1, '\u2502', nil, borderStyle)
	t.screen.SetContent(x+1, y+1, suitRune, nil, suitStyle)

	rankStr := rankToShort(card.Rank)
	for i, ch := range rankStr {
		if i < 3 {
			t.screen.SetContent(x+2+i, y+1, ch, nil, textStyle)
		}
	}
	for i := 2 + len(rankStr); i < 4; i++ {
		t.screen.SetContent(x+i, y+1, ' ', nil, textStyle)
	}
	t.screen.SetContent(x+4, y+1, '\u2502', nil, borderStyle)

	// Bottom border
	t.screen.SetContent(x, y+2, '\u2514', nil, borderStyle)
	t.screen.SetContent(x+1, y+2, '\u2500', nil, borderStyle)
	t.screen.SetContent(x+2, y+2, '\u2500', nil, borderStyle)
	t.screen.SetContent(x+3, y+2, '\u2500', nil, borderStyle)
	t.screen.SetContent(x+4, y+2, '\u2518', nil, borderStyle)
}

// drawCardBack draws a face-down card at (x, y), 5 wide x 3 tall
func (t *TUI) drawCardBack(x, y int) {
	style := tcell.StyleDefault.Foreground(tcell.ColorDarkCyan).Background(tcell.NewRGBColor(0, 40, 60))
	// Top border
	t.screen.SetContent(x, y, '┌', nil, style)
	t.screen.SetContent(x+1, y, '─', nil, style)
	t.screen.SetContent(x+2, y, '─', nil, style)
	t.screen.SetContent(x+3, y, '─', nil, style)
	t.screen.SetContent(x+4, y, '┐', nil, style)
	// Content
	t.screen.SetContent(x, y+1, '│', nil, style)
	t.screen.SetContent(x+1, y+1, '░', nil, style)
	t.screen.SetContent(x+2, y+1, '░', nil, style)
	t.screen.SetContent(x+3, y+1, '░', nil, style)
	t.screen.SetContent(x+4, y+1, '│', nil, style)
	// Bottom border
	t.screen.SetContent(x, y+2, '└', nil, style)
	t.screen.SetContent(x+1, y+2, '─', nil, style)
	t.screen.SetContent(x+2, y+2, '─', nil, style)
	t.screen.SetContent(x+3, y+2, '─', nil, style)
	t.screen.SetContent(x+4, y+2, '┘', nil, style)
}

// drawCardsRow draws a row of cards (for AI plays)
func (t *TUI) drawCardsRow(x, y int, cards []Card, selectable bool) {
	for i, c := range cards {
		t.drawCard(x+i*6, y, c, false, IsTrump(c, t.game.TrumpSuit, t.game.DealerLevel()), false)
	}
}

// drawActionButtons draws action buttons at the bottom
func (t *TUI) drawActionButtons() {
	y := t.height - 1

	switch t.phase {
	case UIPhaseWelcome:
		t.addButton("[Enter:开始游戏]", "start", t.width/2-5, y)
	case UIPhaseBidding:
		hint := "按B亮主/P不亮，或点击按钮"
		t.drawString(t.width/2-len(hint)/2, y-1, hint, tcell.StyleDefault.Foreground(tcell.ColorGreen))
		t.addButton("[B:亮主]", "bid", t.width/2-8, y)
		t.addButton("[P:不亮]", "pass", t.width/2+2, y)
	case UIPhasePlaying:
		needCount := 1
		if t.game.CurrentTrick != nil && t.game.CurrentTrick.PlayerCount() > 0 {
			needCount = len(t.game.CurrentTrick.LeadCards())
		}
		selectedCount := len(t.selected)
		hintStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
		hint := fmt.Sprintf("双击直接出牌 或 点击选中(已选%d张/需选%d张) P:出牌 C:取消", selectedCount, needCount)
		t.drawString(t.width/2-len(hint)/2, y-1, hint, hintStyle)
		t.addButton("[Enter/P:出牌]", "play", t.width/2-14, y)
		t.addButton("[C:取消]", "cancel", t.width/2+4, y)
	case UIPhaseDiscard:
		selectedCount := len(t.selected)
		hintStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
		hint := fmt.Sprintf("点击牌选中(已选%d张/需扣%d张) D:扣底 C:取消", selectedCount, t.discardCount)
		t.drawString(t.width/2-len(hint)/2, y-1, hint, hintStyle)
		t.addButton("[Enter/D:扣底]", "play", t.width/2-14, y)
		t.addButton("[C:取消]", "cancel", t.width/2+6, y)
		case UIPhaseWaitTrick:
			// Result shown in center box, bottom bar empty
		case UIPhaseHandResult:
			t.drawString(t.width/2-12, y, "5秒后自动开始下一局(按键跳过)", tcell.StyleDefault.Foreground(tcell.ColorYellow))
	case UIPhaseGameOver:
		t.drawString(t.width/2-4, y, "游戏结束!", tcell.StyleDefault.Bold(true).Foreground(tcell.ColorYellow))
	}
}

// addButton adds a clickable button
func (t *TUI) addButton(label, action string, x, y int) {
	btnStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow).Bold(true)
	wx := 0
	for _, ch := range label {
		t.screen.SetContent(x+wx, y, ch, nil, btnStyle)
		wx += runewidth.RuneWidth(ch)
	}
	t.buttons = append(t.buttons, Button{Label: label, Action: action, X: x, Y: y, W: runewidth.StringWidth(label)})
}

// drawMessage draws a message box in the center
func (t *TUI) drawMessage() {
	if t.message == "" {
		return
	}

	// Draw message box
	lines := splitLines(t.message)
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	boxW := maxLen + 4
	boxH := len(lines) + 2 + len(t.msgButtons)
	boxX := (t.width - boxW) / 2
	boxY := (t.height - boxH) / 2

	// Background
	bgStyle := tcell.StyleDefault.Background(tcell.ColorNavy)
	for y := boxY; y < boxY+boxH; y++ {
		for x := boxX; x < boxX+boxW; x++ {
			t.screen.SetContent(x, y, ' ', nil, bgStyle)
		}
	}

	// Border
	borderStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorNavy)
	for x := boxX; x < boxX+boxW; x++ {
		t.screen.SetContent(x, boxY, '─', nil, borderStyle)
		t.screen.SetContent(x, boxY+boxH-1, '─', nil, borderStyle)
	}
	for y := boxY; y < boxY+boxH; y++ {
		t.screen.SetContent(boxX, y, '│', nil, borderStyle)
		t.screen.SetContent(boxX+boxW-1, y, '│', nil, borderStyle)
	}
	t.screen.SetContent(boxX, boxY, '┌', nil, borderStyle)
	t.screen.SetContent(boxX+boxW-1, boxY, '┐', nil, borderStyle)
	t.screen.SetContent(boxX, boxY+boxH-1, '└', nil, borderStyle)
	t.screen.SetContent(boxX+boxW-1, boxY+boxH-1, '┘', nil, borderStyle)

	// Text
	textStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorNavy)
	for i, line := range lines {
		t.drawString(boxX+2, boxY+1+i, line, textStyle)
	}

	// Buttons
	for i, b := range t.msgButtons {
		btnY := boxY + len(lines) + 1 + i
		t.screen.SetContent(boxX+2, btnY, ' ', nil, bgStyle)
		btnStyle := tcell.StyleDefault.Reverse(true).Background(tcell.ColorNavy)
		for j, ch := range b.Label {
			t.screen.SetContent(boxX+3+j, btnY, ch, nil, btnStyle)
		}
		t.msgButtons[i].X = boxX + 3
		t.msgButtons[i].Y = btnY
		t.msgButtons[i].W = len(b.Label)
	}
}

// ============ Helper Functions ============

func (t *TUI) drawString(x, y int, s string, style tcell.Style) {
	for i, ch := range s {
		t.screen.SetContent(x+i, y, ch, nil, style)
	}
}

func suitToRune(s Suit) rune {
	switch s {
	case SuitSpade:
		return '♠'
	case SuitHeart:
		return '♥'
	case SuitDiamond:
		return '♦'
	case SuitClub:
		return '♣'
	case SuitJoker:
		return '★'
	}
	return '?'
}

func rankToShort(r Rank) string {
	switch r {
	case Rank10:
		return "10"
	case RankJ:
		return "J"
	case RankQ:
		return "Q"
	case RankK:
		return "K"
	case RankA:
		return "A"
	case RankSmallJoker:
		return "小"
	case RankBigJoker:
		return "大"
	default:
		return r.String()
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, ch := range s {
		if ch == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
