package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// main関数は、サブコマンドのルーターとして機能します。
func main() {
	// サブコマンドが指定されているかチェック
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [arguments]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Available commands: transform, split\n")
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

	case "split": // *** "split"コマンドの処理を新設 ***
		if len(os.Args) != 6 {
			fmt.Fprintf(os.Stderr, "Usage: %s split <split_tag> <max_kb> <input.xml> <output_prefix>\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "  <split_tag>: 分割の単位となるタグ名 (例: item)\n")
			fmt.Fprintf(os.Stderr, "  <max_kb>: 1ファイルあたりのおおよその最大サイズ (KB)\n")
			fmt.Fprintf(os.Stderr, "  <input.xml>: 分割対象のXMLファイル\n")
			fmt.Fprintf(os.Stderr, "  <output_prefix>: 出力ファイル名の接頭辞 (例: output/split_)\n")
			os.Exit(1)
		}

		splitTag := os.Args[2]
		maxKB, err := strconv.Atoi(os.Args[3])
		if err != nil {
			log.Fatalf("Error: <max_kb> must be a number: %v", err)
		}
		inputFilepath := os.Args[4]
		outputPrefix := os.Args[5]

		if err := runSplit(splitTag, maxKB, inputFilepath, outputPrefix); err != nil {
			log.Fatalf("Error during split: %v", err)
		}

		fmt.Println("XML splitting completed.")

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: '%s'\n", subcommand)
		fmt.Fprintf(os.Stderr, "Available commands: transform, split\n")
		os.Exit(1)
	}
}
