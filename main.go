package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// main関数は、サブコマンドのルーターとして機能します。
func main() {
	// サブコマンドが指定されているかチェック
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [arguments]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Available commands: transform, newline, maxline, remove, noline, sort\n")
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
			fmt.Fprintf(os.Stderr, "Usage: %s newline <target_strings> <position> <input_file> <output_file>\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "  <target_strings>: カンマ区切りのターゲット文字列 (例: \"<a>,<b>\")\n")
			fmt.Fprintf(os.Stderr, "  <position>      : 'before' or 'after'\n")
			os.Exit(1)
		}
		// カンマ区切りでスライスに変換
		targetArg := os.Args[2]
		targets := strings.Split(targetArg, ",")

		position := os.Args[3]
		inputFile := os.Args[4]
		outputFile := os.Args[5]

		if err := runNewline(targets, position, inputFile, outputFile); err != nil {
			log.Fatalf("Error during newline processing: %v", err)
		}
		fmt.Println("Newline processing completed.")

	case "maxline":
		if len(os.Args) != 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s maxline <input_file>\n", os.Args[0])
			os.Exit(1)
		}
		inputFile := os.Args[2]

		if err := runMaxLine(inputFile); err != nil {
			log.Fatalf("Error during maxline processing: %v", err)
		}

	case "remove":
		if len(os.Args) != 5 {
			fmt.Fprintf(os.Stderr, "Usage: %s remove <target_strings> <input_file> <output_file>\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "  <target_strings>: Comma-separated strings to remove (e.g. \"foo,bar\")\n")
			os.Exit(1)
		}

		// カンマ区切りで分割してスライスにする
		targetArg := os.Args[2]
		targets := strings.Split(targetArg, ",")

		inputFile := os.Args[3]
		outputFile := os.Args[4]

		if err := runRemove(targets, inputFile, outputFile); err != nil {
			log.Fatalf("Error during remove processing: %v", err)
		}
		fmt.Println("Remove processing completed.")

	case "noline":
		if len(os.Args) != 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s noline <input_file> <output_file>\n", os.Args[0])
			os.Exit(1)
		}
		inputFile := os.Args[2]
		outputFile := os.Args[3]

		if err := runNoLine(inputFile, outputFile); err != nil {
			log.Fatalf("Error during noline processing: %v", err)
		}
		fmt.Println("NoLine processing completed.")

	case "sort":
		// 引数は 4つ (デフォルトメモリ) または 5つ (メモリ指定あり)
		if len(os.Args) < 4 || len(os.Args) > 5 {
			fmt.Fprintf(os.Stderr, "Usage: %s sort <input_file> <output_file> [memory_limit_mb]\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "  [memory_limit_mb]: Max memory usage in MB (default: 512)\n")
			os.Exit(1)
		}
		inputFile := os.Args[2]
		outputFile := os.Args[3]

		// デフォルト 512MB
		memoryLimitMB := 512

		// 第4引数があればパースして上書き
		if len(os.Args) == 5 {
			var err error
			memoryLimitMB, err = strconv.Atoi(os.Args[4])
			if err != nil || memoryLimitMB <= 0 {
				log.Fatalf("Error: <memory_limit_mb> must be a positive number: %v", err)
			}
		}

		// MB を Byte に変換
		memoryLimitBytes := int64(memoryLimitMB) * 1024 * 1024

		if err := runSort(inputFile, outputFile, memoryLimitBytes); err != nil {
			log.Fatalf("Error during sort processing: %v", err)
		}
		fmt.Printf("Sort processing completed. (Max Memory: %d MB)\n", memoryLimitMB)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: '%s'\n", subcommand)
		fmt.Fprintf(os.Stderr, "Available commands: transform, newline, maxline, remove, noline, sort\n")
		os.Exit(1)
	}
}
