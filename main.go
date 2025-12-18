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
		fmt.Fprintf(os.Stderr, "Available commands: transform, newline\n")
		os.Exit(1)
	}

	// 最初の引数をサブコマンドとして解釈
	subcommand := os.Args[1]

	// サブコマンドに応じて処理を分岐
	switch subcommand {
	case "transform":
		// transform コマンドの引数が正しいかチェック (program + transform + rules + input + output = 5)
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

	case "newline":
		if len(os.Args) != 6 {
			fmt.Fprintf(os.Stderr, "Usage: %s newline <target_string> <position> <input_file> <output_file>\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "  <target_string>: 改行の基準となる文字列\n")
			fmt.Fprintf(os.Stderr, "  <position>     : 'before' or 'after'\n")
			os.Exit(1)
		}
		targetString := os.Args[2]
		position := os.Args[3]
		inputFile := os.Args[4]
		outputFile := os.Args[5]

		if err := runNewline(targetString, position, inputFile, outputFile); err != nil {
			log.Fatalf("Error during newline processing: %v", err)
		}
		fmt.Println("Newline processing completed.")

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: '%s'\n", subcommand)
		fmt.Fprintf(os.Stderr, "Available commands: transform, newline\n")
		os.Exit(1)
	}
}
