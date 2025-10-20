package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// processor は、XML処理のロジックと状態を保持します。
type processor struct {
	decoder *xml.Decoder
	encoder *xml.Encoder

	nameRules   []NameReplaceRule
	insertRules []InsertBeforeRule
	valueRules  []ValueReplaceRule
	wrapRuleMap map[string]string

	elementStack []xml.StartElement
}

// newProcessor は、新しいprocessorを初期化します。
func newProcessor(r io.Reader, w io.Writer, nameRules []NameReplaceRule, insertRules []InsertBeforeRule, valueRules []ValueReplaceRule, wrapRules []WrapRule) *processor { // *** wrapRulesを追加 ***
	decoder := xml.NewDecoder(r)
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")

	// WrapRuleを高速に検索できるようMapに変換
	wrapMap := make(map[string]string)
	for _, rule := range wrapRules {
		wrapMap[rule.TargetTag] = rule.WrapperTag
	}

	return &processor{
		decoder:      decoder,
		encoder:      encoder,
		nameRules:    nameRules,
		insertRules:  insertRules,
		valueRules:   valueRules,
		wrapRuleMap:  wrapMap,
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
	// 1. 前方挿入ルール
	for _, rule := range p.insertRules {
		if se.Name.Local == rule.TargetTag {
			var xmlFragment string
			if rule.Counter != nil {
				count := rule.Counter.Next()
				xmlFragment = fmt.Sprintf(rule.XMLTemplate, count)
			} else {
				xmlFragment = rule.XMLTemplate
			}

			fragmentDecoder := xml.NewDecoder(strings.NewReader(xmlFragment))
			for {
				token, err := fragmentDecoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("failed to decode fragment: %w", err)
				}
				if err := p.encoder.EncodeToken(token); err != nil {
					return fmt.Errorf("failed to encode fragment token: %w", err)
				}
			}
		}
	}

	// 2. タグ名置換ルール
	processedSE := se
	for _, rule := range p.nameRules {
		if processedSE.Name.Local == rule.OldName {
			processedSE.Name.Local = rule.NewName
			break
		}
	}

	// 3. 実際の開始タグを書き込む
	if err := p.encoder.EncodeToken(processedSE); err != nil {
		return fmt.Errorf("failed to encode start element: %w", err)
	}
	p.elementStack = append(p.elementStack, processedSE)

	// 4. 子のラップ開始ルール
	// 対象タグの開始タグを書き込んだ直後に、ラッパーの開始タグを書き込む
	if wrapperTag, found := p.wrapRuleMap[processedSE.Name.Local]; found {
		wrapperSE := xml.StartElement{Name: xml.Name{Local: wrapperTag}}
		if err := p.encoder.EncodeToken(wrapperSE); err != nil {
			return err
		}
	}
	return nil
}

// handleCharData は、テキストデータを処理します。
func (p *processor) handleCharData(cd xml.CharData) error {
	// 自動インデント機能が有効なため、元のファイルにあるフォーマット用の
	// 空白文字（改行やインデントのみのテキスト）は破棄する。
	if len(strings.TrimSpace(string(cd))) == 0 {
		return nil // 空白のみのテキストノードはここで処理を終了
	}

	if len(p.elementStack) > 0 {
		currentElement := p.elementStack[len(p.elementStack)-1]
		for _, rule := range p.valueRules {
			if currentElement.Name.Local == rule.TargetTag {
				oldValue := string(cd)
				newValue := rule.ReplacementFunc(oldValue)
				return p.encoder.EncodeToken(xml.CharData(newValue))
			}
		}
	}
	return p.encoder.EncodeToken(cd)
}

// handleEndElement は、終了タグを処理します。
func (p *processor) handleEndElement(_ xml.EndElement) error {
	if len(p.elementStack) == 0 {
		return fmt.Errorf("invalid XML structure")
	}

	// スタックから対応する開始タグの情報を取り出す
	lastStartedElem := p.elementStack[len(p.elementStack)-1]
	p.elementStack = p.elementStack[:len(p.elementStack)-1]

	// 1. 子のラップ終了ルール
	// 対象タグの終了タグを書き込む直前に、ラッパーの終了タグを書き込む
	if wrapperTag, found := p.wrapRuleMap[lastStartedElem.Name.Local]; found {
		wrapperEE := xml.EndElement{Name: xml.Name{Local: wrapperTag}}
		if err := p.encoder.EncodeToken(wrapperEE); err != nil {
			return err
		}
	}

	// 2. 実際の終了タグを書き込む
	if err := p.encoder.EncodeToken(xml.EndElement{Name: lastStartedElem.Name}); err != nil {
		return err
	}
	return nil
}
