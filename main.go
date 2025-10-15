package main

import (
	"fmt"
	"log"
	"os"
)

// main関数は、サブコマンドのルーターとして機能します。
func main() {
	// サブコマンドが指定されているかチェック
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [arguments]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Available commands: transform\n")
		os.Exit(1)
	}

	// 最初の引数をサブコマンドとして解釈
	subcommand := os.Args[1]

	// サブコマンドに応じて処理を分岐
	switch subcommand {
	case "transform":
		// transform コマンドの引数が正しいかチェック (program + transform + input + output = 4)
		if len(os.Args) != 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s transform <input.xml> <output.xml>\n", os.Args[0])
			os.Exit(1)
		}
		inputFilepath := os.Args[2]
		outputFilepath := os.Args[3]

		// XML変換処理を実行
		if err := runTransform(inputFilepath, outputFilepath); err != nil {
			log.Fatalf("Error during transform: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: '%s'\n", subcommand)
		fmt.Fprintf(os.Stderr, "Available commands: transform\n")
		os.Exit(1)
	}
}
