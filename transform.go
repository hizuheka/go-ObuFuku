package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// runTransform は、ルールファイルに基づいてXML変換処理を実行します。
func runTransform(ruleFilepath, inputFilepath, outputFilepath string) error {
	// --- ルールファイルの読み込みと組み立て (この部分は変更ありません) ---
	ruleFile, err := os.ReadFile(ruleFilepath)
	if err != nil {
		return fmt.Errorf("failed to read rule file '%s': %w", ruleFilepath, err)
	}
	var config Config
	if err := json.Unmarshal(ruleFile, &config); err != nil {
		return fmt.Errorf("failed to parse rule file '%s': %w", ruleFilepath, err)
	}
	counters := make(map[string]*Counter)
	for name, counterConfig := range config.Counters {
		counters[name] = &Counter{current: counterConfig.Start}
	}
	var nameRules []NameReplaceRule
	for _, r := range config.NameRules {
		nameRules = append(nameRules, NameReplaceRule{OldName: r.Old, NewName: r.New})
	}
	var insertRules []InsertBeforeRule
	for _, r := range config.InsertRules {
		insertRules = append(insertRules, InsertBeforeRule{
			TargetTag:   r.Target,
			XMLTemplate: r.Template,
			Counter:     counters[r.Counter],
		})
	}
	var valueRules []ValueReplaceRule
	for _, r := range config.ValueRules {
		replaceFunc, err := buildValueReplaceFunc(r)
		if err != nil {
			return err
		}
		valueRules = append(valueRules, ValueReplaceRule{
			TargetTag:       r.Target,
			ReplacementFunc: replaceFunc,
		})
	}
	var wrapRules []WrapRule
	for _, r := range config.WrapRules {
		wrapRules = append(wrapRules, WrapRule{TargetTag: r.Target, WrapperTag: r.Wrapper})
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

	// 出力ファイルをcrlfWriterでラップする
	writer := newCRLFWriter(outputFile)

	// --- プロセッサの実行 ---
	// ラップしたwriterをプロセッサに渡す
	proc := newProcessor(inputFile, writer, nameRules, insertRules, valueRules, wrapRules)
	if err := proc.Run(); err != nil {
		return fmt.Errorf("error processing XML: %w", err)
	}

	fmt.Printf("XML processing completed. Rules: '%s', Input: '%s', Output: '%s'\n", ruleFilepath, inputFilepath, outputFilepath)
	return nil
}
