package main

import (
	"fmt"
	"os"
)

// runTransform は、XML変換処理の本体です。
func runTransform(inputFilepath, outputFilepath string) error {
	// --- 適用するルールをここで定義 ---
	nameRules := []NameReplaceRule{
		{OldName: "database", NewName: "records"},
		{OldName: "document", NewName: "item"},
		{OldName: "legacy_user", NewName: "user"},
	}

	insertionCounter := &Counter{current: 0}
	insertRules := []InsertBeforeRule{
		{
			TargetTag:   "status",
			XMLTemplate: "<A>value1</A><B>%d</B>",
			Counter:     insertionCounter,
		},
	}

	valueRules := []ValueReplaceRule{
		{
			TargetTag: "id",
			ReplacementFunc: func(oldValue string) string {
				return "00000" + oldValue
			},
		},
		{
			TargetTag: "data",
			ReplacementFunc: func(oldValue string) string {
				return oldValue + "_processed"
			},
		},
	}

	// --- ファイルの準備 ---
	inputFile, err := os.Open(inputFilepath)
	if err != nil {
		return fmt.Errorf("error opening input file '%s': %w", inputFilepath, err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputFilepath)
	if err != nil {
		return fmt.Errorf("error creating output file '%s': %w", outputFilepath, err)
	}
	defer outputFile.Close()

	// --- プロセッサの実行 ---
	proc := newProcessor(inputFile, outputFile, nameRules, insertRules, valueRules)
	if err := proc.Run(); err != nil {
		return fmt.Errorf("error processing XML: %w", err)
	}

	fmt.Printf("XML processing completed. Input: '%s', Output: '%s'\n", inputFilepath, outputFilepath)
	return nil
}
