package main

import (
	"fmt"
	"os"
)

func main() {
	game := NewGame()
	tui := NewTUI(game)

	if err := tui.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "无法初始化终端界面: %v\n", err)
		fmt.Fprintf(os.Stderr, "请确保在终端中运行此程序\n")
		os.Exit(1)
	}
	defer tui.Close()

	tui.Run()
}
