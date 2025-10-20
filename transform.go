package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// runTransform は、ルールファイルに基づいてXML変換処理を実行します。
func runTransform(ruleFilepath, inputFilepath, outputFilepath string) error {
	// --- ルールファイルの読み込み ---
	ruleFile, err := os.ReadFile(ruleFilepath)
	if err != nil {
		return fmt.Errorf("failed to read rule file '%s': %w", ruleFilepath, err)
	}

	var config Config
	if err := json.Unmarshal(ruleFile, &config); err != nil {
		return fmt.Errorf("failed to parse rule file '%s': %w", ruleFilepath, err)
	}

	// --- JSON設定から実行用ルールを組み立て ---

	// カウンターの準備
	counters := make(map[string]*Counter)
	for name, counterConfig := range config.Counters {
		counters[name] = &Counter{current: counterConfig.Start}
	}

	// NameRules の組み立て
	var nameRules []NameReplaceRule
	for _, r := range config.NameRules {
		nameRules = append(nameRules, NameReplaceRule{OldName: r.Old, NewName: r.New})
	}

	// InsertRules の組み立て
	var insertRules []InsertBeforeRule
	for _, r := range config.InsertRules {
		insertRules = append(insertRules, InsertBeforeRule{
			TargetTag:   r.Target,
			XMLTemplate: r.Template,
			Counter:     counters[r.Counter],
		})
	}

	// InsertAfterRules の組み立て
	var insertAfterRules []InsertBeforeRule
	for _, r := range config.InsertAfterRules {
		insertAfterRules = append(insertAfterRules, InsertBeforeRule{
			TargetTag:   r.Target,
			XMLTemplate: r.Template,
			Counter:     counters[r.Counter],
		})
	}

	// PrependChildRules の組み立て
	var prependChildRules []InsertBeforeRule
	for _, r := range config.PrependChildRules {
		prependChildRules = append(prependChildRules, InsertBeforeRule{
			TargetTag:   r.Target,
			XMLTemplate: r.Template,
			Counter:     counters[r.Counter],
		})
	}

	// ValueRules の組み立て
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

	// WrapRules の組み立て
	var wrapRules []WrapRule
	for _, r := range config.WrapRules {
		wrapRules = append(wrapRules, WrapRule{TargetTag: r.Target, WrapperTag: r.Wrapper})
	}

	// CdataRules の組み立て
	var cdataRules []CdataRule
	for _, r := range config.CdataRules {
		cdataRules = append(cdataRules, CdataRule{Old: r.Old, New: r.New})
	}

	// RawTags はそのままスライスとして使う
	rawTags := config.RawTags

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

	// CRLF改行コードを強制するwriterでラップ
	writer := newCRLFWriter(outputFile)

	// --- プロセッサの実行 ---
	proc := newProcessor(inputFile, writer, nameRules, insertRules, insertAfterRules, prependChildRules, valueRules, wrapRules, cdataRules, rawTags)
	if err := proc.Run(); err != nil {
		return fmt.Errorf("error processing XML: %w", err)
	}

	fmt.Printf("XML processing completed. Rules: '%s', Input: '%s', Output: '%s'\n", ruleFilepath, inputFilepath, outputFilepath)
	return nil
}
