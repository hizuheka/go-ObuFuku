package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// --- ルールの定義 (変更なし) ---
// ... (ここから processor 構造体と các メソッドの定義までは変更ありません) ...

// NameReplaceRule は、タグ名を置換するためのルールです。
type NameReplaceRule struct {
	OldName string
	NewName string
}

// InsertBeforeRule は、指定したタグの直前にXML断片を挿入するためのルールです。
type InsertBeforeRule struct {
	TargetTag   string
	XMLFragment string // 挿入するXML文字列
}

// ValueReplaceRule は、タグの値を置換するためのルールです。
type ValueReplaceRule struct {
	TargetTag string
	NewValue  string
}

// processor は、XML処理のロジックと状態を保持します。
type processor struct {
	decoder *xml.Decoder
	encoder *xml.Encoder

	// 適用するルールのスライス
	nameRules   []NameReplaceRule
	insertRules []InsertBeforeRule
	valueRules  []ValueReplaceRule

	// 現在のXML階層を追跡するためのスタック
	elementStack []xml.StartElement
}

// newProcessor は、新しいprocessorを初期化します。
func newProcessor(r io.Reader, w io.Writer, nameRules []NameReplaceRule, insertRules []InsertBeforeRule, valueRules []ValueReplaceRule) *processor {
	decoder := xml.NewDecoder(r)
	encoder := xml.NewEncoder(w)

	return &processor{
		decoder:      decoder,
		encoder:      encoder,
		nameRules:    nameRules,
		insertRules:  insertRules,
		valueRules:   valueRules,
		elementStack: make([]xml.StartElement, 0),
	}
}

// Run は、XMLの処理を実行します。
func (p *processor) Run() error {
	for {
		token, err := p.decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to get token: %w", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			if err := p.handleStartElement(elem); err != nil {
				return err
			}
		case xml.CharData:
			if err := p.handleCharData(elem); err != nil {
				return err
			}
		case xml.EndElement:
			if err := p.handleEndElement(elem); err != nil {
				return err
			}
		default:
			if err := p.encoder.EncodeToken(elem); err != nil {
				return fmt.Errorf("failed to encode token: %w", err)
			}
		}
	}
	return p.encoder.Flush()
}

// handleStartElement は、開始タグを処理します。
func (p *processor) handleStartElement(se xml.StartElement) error {
	for _, rule := range p.insertRules {
		if se.Name.Local == rule.TargetTag {
			if err := p.encoder.Flush(); err != nil {
				return err
			}
			if _, err := io.WriteString(p.encoder.Writer.(io.Writer), rule.XMLFragment); err != nil {
				return err
			}
		}
	}

	processedSE := se
	for _, rule := range p.nameRules {
		if processedSE.Name.Local == rule.OldName {
			processedSE.Name.Local = rule.NewName
			break
		}
	}

	p.elementStack = append(p.elementStack, processedSE)

	if err := p.encoder.EncodeToken(processedSE); err != nil {
		return fmt.Errorf("failed to encode start element: %w", err)
	}
	return nil
}

// handleCharData は、テキストデータを処理します。
func (p *processor) handleCharData(cd xml.CharData) error {
	isWhitespaceOnly := len(strings.TrimSpace(string(cd))) == 0

	if len(p.elementStack) > 0 && !isWhitespaceOnly {
		currentElement := p.elementStack[len(p.elementStack)-1]

		for _, rule := range p.valueRules {
			if currentElement.Name.Local == rule.TargetTag {
				return p.encoder.EncodeToken(xml.CharData(rule.NewValue))
			}
		}
	}
	return p.encoder.EncodeToken(cd)
}

// handleEndElement は、終了タグを処理します。
func (p *processor) handleEndElement(_ xml.EndElement) error {
	if len(p.elementStack) == 0 {
		return fmt.Errorf("invalid XML structure: found end element without matching start element")
	}

	lastStartedElem := p.elementStack[len(p.elementStack)-1]
	p.elementStack = p.elementStack[:len(p.elementStack)-1]

	if err := p.encoder.EncodeToken(xml.EndElement{Name: lastStartedElem.Name}); err != nil {
		return fmt.Errorf("failed to encode end element: %w", err)
	}
	return nil
}


// *** ここから下が修正された main 関数です ***
func main() {
	// --- 引数のチェック ---
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.xml> <output.xml>\n", os.Args[0])
		os.Exit(1)
	}
	inputFilepath := os.Args[1]
	outputFilepath := os.Args[2]


	// --- 適用するルールをここで定義 ---
	nameRules := []NameReplaceRule{
		{OldName: "database", NewName: "records"},
		{OldName: "document", NewName: "item"},
		{OldName: "legacy_user", NewName: "user"},
	}
	insertRules := []InsertBeforeRule{
		{TargetTag: "status", XMLFragment: "\n    <processed_by>go_processor</processed_by>"},
	}
	valueRules := []ValueReplaceRule{
		{TargetTag: "data", NewValue: "REDACTED"},
	}

	// --- ファイルの準備 ---
	inputFile, err := os.Open(inputFilepath)
	if err != nil {
		log.Fatalf("Error opening input file '%s': %v", inputFilepath, err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputFilepath)
	if err != nil {
		log.Fatalf("Error creating output file '%s': %v", outputFilepath, err)
	}
	defer outputFile.Close()

	// --- プロセッサの実行 ---
	proc := newProcessor(inputFile, outputFile, nameRules, insertRules, valueRules)
	if err := proc.Run(); err != nil {
		log.Fatalf("Error processing XML: %v", err)
	}

	fmt.Printf("XML processing completed. Input: '%s', Output: '%s'\n", inputFilepath, outputFilepath)
}
