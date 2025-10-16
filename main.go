package main

import (
	"fmt"
	"log"
	"os"
)

// main関数は、サブコマンドのルーターとして機能します。
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [arguments]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Available commands: transform\n")
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "transform":
		// 引数の数をチェック (program + transform + rules + input + output = 5)
		if len(os.Args) != 5 {
			fmt.Fprintf(os.Stderr, "Usage: %s transform <rules.json> <input.xml> <output.xml>\n", os.Args[0])
			os.Exit(1)
		}
		ruleFilepath := os.Args[2]
		inputFilepath := os.Args[3]
		outputFilepath := os.Args[4]

		// XML変換処理を実行
		if err := runTransform(ruleFilepath, inputFilepath, outputFilepath); err != nil {
			log.Fatalf("Error during transform: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: '%s'\n", subcommand)
		fmt.Fprintf(os.Stderr, "Available commands: transform\n")
		os.Exit(1)
	}
}
