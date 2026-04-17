package main

import "fmt"

// Display functions for the CLI interface

// DisplayGameState shows the current game state
func DisplayGameState(game *Game) {
	level := game.DealerLevel()

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Printf("║  升级（拖拉机）  庄家打%-4s          ║\n", LevelDisplayName(level))
	if game.TrumpSuit != SuitJoker {
		fmt.Printf("║  主牌花色：%-6s                   ║\n", game.TrumpSuit.String())
	} else {
		fmt.Printf("║  无主                                ║\n")
	}
	fmt.Printf("║  庄家：%s    闲家：%s      ║\n",
		formatTeam(game.DealerTeam()),
		formatTeam(game.OpponentTeam()))
	fmt.Printf("║  南北级：%s  东西级：%s         ║\n",
		LevelDisplayName(game.Level[Team0]),
		LevelDisplayName(game.Level[Team1]))
	fmt.Println("╚══════════════════════════════════════╝")
}

// DisplayScore shows the current score
func DisplayScore(game *Game) {
	fmt.Println()
	fmt.Println("┌────────── 得分 ──────────┐")
	fmt.Printf("│ 南北方：%4d分            │\n", game.TeamScore[Team0])
	fmt.Printf("│ 东西方：%4d分            │\n", game.TeamScore[Team1])
	fmt.Printf("│ 已完成：%d/25轮          │\n", game.TrickCount)
	fmt.Println("└──────────────────────────┘")
}

// DisplayTableLayout shows the table with player positions
func DisplayTableLayout(game *Game) {
	fmt.Println()
	fmt.Println("              北(AI)")
	fmt.Println("            ┌──────┐")
	fmt.Println("  西(AI) ──┤  桌面  ├── 东(AI)")
	fmt.Println("            └──────┘")
	fmt.Println("              南(你)")
}

// DisplayWelcome shows the welcome message
func DisplayWelcome() {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════╗")
	fmt.Println("  ║                                      ║")
	fmt.Println("  ║      升 级 （拖 拉 机）纸 牌 游 戏    ║")
	fmt.Println("  ║                                      ║")
	fmt.Println("  ║      两副牌  ·  2为常主  ·  4人对战   ║")
	fmt.Println("  ║                                      ║")
	fmt.Println("  ║  规则简述：                           ║")
	fmt.Println("  ║  - 你坐南方，与北方AI搭档             ║")
	fmt.Println("  ║  - 西方AI与东方AI搭档                 ║")
	fmt.Println("  ║  - 抢分升级，先升到A方获胜            ║")
	fmt.Println("  ║  - 5=5分 10=10分 K=10分               ║")
	fmt.Println("  ║  - 出牌时输入牌的序号，空格分隔        ║")
	fmt.Println("  ║                                      ║")
	fmt.Println("  ╚══════════════════════════════════════╝")
	fmt.Println()
	fmt.Print("按回车键开始游戏...")
	var input string
	fmt.Scanln(&input)
}

// DisplayTrumpInfo shows trump information
func DisplayTrumpInfo(trumpSuit Suit, level Rank) {
	fmt.Println()
	fmt.Println("┌────────── 主牌信息 ──────────┐")
	if trumpSuit == SuitJoker {
		fmt.Println("│ 本局打无主                    │")
		fmt.Println("│ 主牌：大小王 + 所有级牌 + 所有2 │")
	} else {
		fmt.Printf("│ 主牌花色：%s                 │\n", trumpSuit.String())
		fmt.Println("│ 主牌：大小王 > 主级牌 > 副级牌 │")
		fmt.Println("│       > 主2 > 副2 > 主A>...>3 │")
	}
	fmt.Printf("│ 级牌：%s                      │\n", level.String())
	fmt.Println("└──────────────────────────────┘")
}
