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
	writer  io.Writer

	nameRules         []NameReplaceRule
	insertRules       []InsertBeforeRule
	insertAfterRules  []InsertBeforeRule
	prependChildRules []InsertBeforeRule
	valueRules        []ValueReplaceRule
	wrapRuleMap       map[string]string
	cdataRules        []CdataRule
	rawTagMap         map[string]bool

	elementStack []xml.StartElement
}

// newProcessor は、新しいprocessorを初期化します。
func newProcessor(r io.Reader, w io.Writer, nameRules []NameReplaceRule, insertRules []InsertBeforeRule, insertAfterRules []InsertBeforeRule, prependChildRules []InsertBeforeRule, valueRules []ValueReplaceRule, wrapRules []WrapRule, cdataRules []CdataRule, rawTags []string) *processor {
	decoder := xml.NewDecoder(r)
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")

	wrapMap := make(map[string]string)
	for _, rule := range wrapRules {
		wrapMap[rule.TargetTag] = rule.WrapperTag
	}

	rawMap := make(map[string]bool)
	for _, tag := range rawTags {
		rawMap[tag] = true
	}

	return &processor{
		decoder:           decoder,
		encoder:           encoder,
		writer:            w,
		nameRules:         nameRules,
		insertRules:       insertRules,
		insertAfterRules:  insertAfterRules,
		prependChildRules: prependChildRules,
		valueRules:        valueRules,
		wrapRuleMap:       wrapMap,
		cdataRules:        cdataRules,
		rawTagMap:         rawMap,
		elementStack:      make([]xml.StartElement, 0),
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
	// 前方挿入ルール
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
					return err
				}
				if err := p.encoder.EncodeToken(token); err != nil {
					return err
				}
			}
		}
	}

	// タグ名置換ルール
	processedSE := se
	for _, rule := range p.nameRules {
		if processedSE.Name.Local == rule.OldName {
			processedSE.Name.Local = rule.NewName
			break
		}
	}

	// 属性値に含まれる余分なダブルクォートを削除
	for i, attr := range processedSE.Attr {
		if len(attr.Value) >= 2 && attr.Value[0] == '"' && attr.Value[len(attr.Value)-1] == '"' {
			processedSE.Attr[i].Value = attr.Value[1 : len(attr.Value)-1]
		}
	}

	// 実際の開始タグを書き込む
	if err := p.encoder.EncodeToken(processedSE); err != nil {
		return err
	}
	p.elementStack = append(p.elementStack, processedSE)

	// 子のラップ開始ルール
	if wrapperTag, found := p.wrapRuleMap[processedSE.Name.Local]; found {
		wrapperSE := xml.StartElement{Name: xml.Name{Local: wrapperTag}}
		if err := p.encoder.EncodeToken(wrapperSE); err != nil {
			return err
		}
	}

	// 子の先頭への挿入ルール
	for _, rule := range p.prependChildRules {
		if processedSE.Name.Local == rule.TargetTag {
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
					return err
				}
				if err := p.encoder.EncodeToken(token); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// handleCharData は、テキストデータを処理します。
func (p *processor) handleCharData(cd xml.CharData) error {
	// 空白のみのテキストノードは破棄
	if len(strings.TrimSpace(string(cd))) == 0 {
		return nil
	}

	// 現在の親タグがraw_tagsで指定されたものかチェック
	isRaw := false
	if len(p.elementStack) > 0 {
		currentElement := p.elementStack[len(p.elementStack)-1]
		if p.rawTagMap[currentElement.Name.Local] {
			isRaw = true
		}
	}

	if isRaw {
		// --- rawタグの中身として処理 ---
		text := string(cd)

		modifiedText := text
		for _, rule := range p.cdataRules {
			modifiedText = strings.ReplaceAll(modifiedText, rule.Old, rule.New)
		}

		// エンコーダーをバイパスして直接書き込む
		if err := p.encoder.Flush(); err != nil {
			return err
		}

		writer := p.writer
		// CDATAで囲むことで、出力されるXMLが壊れるのを防ぐ
		if _, err := io.WriteString(writer, "<![CDATA["); err != nil {
			return err
		}
		if _, err := io.WriteString(writer, modifiedText); err != nil {
			return err
		}
		if _, err := io.WriteString(writer, "]]>"); err != nil {
			return err
		}

		return nil

	} else {
		// --- 通常のタグの中身として処理 ---
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
}

// handleEndElement は、終了タグを処理します。
func (p *processor) handleEndElement(ee xml.EndElement) error {
	if len(p.elementStack) == 0 {
		return fmt.Errorf("invalid XML structure")
	}

	lastStartedElem := p.elementStack[len(p.elementStack)-1]
	p.elementStack = p.elementStack[:len(p.elementStack)-1]

	// 子のラップ終了ルール
	if wrapperTag, found := p.wrapRuleMap[lastStartedElem.Name.Local]; found {
		wrapperEE := xml.EndElement{Name: xml.Name{Local: wrapperTag}}
		if err := p.encoder.EncodeToken(wrapperEE); err != nil {
			return err
		}
	}

	// 実際の終了タグを書き込む
	if err := p.encoder.EncodeToken(xml.EndElement{Name: lastStartedElem.Name}); err != nil {
		return err
	}

	// 後方挿入ルール
	for _, rule := range p.insertAfterRules {
		if ee.Name.Local == rule.TargetTag {
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
					return err
				}
				if err := p.encoder.EncodeToken(token); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
