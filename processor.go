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
	deleteTagMap      map[string]bool

	elementStack []xml.StartElement
	skipDepth    int // スキップ対象のネストの深さ
}

// newProcessor は、新しいprocessorを初期化します。
func newProcessor(r io.Reader, w io.Writer, nameRules []NameReplaceRule, insertRules []InsertBeforeRule, insertAfterRules []InsertBeforeRule, prependChildRules []InsertBeforeRule, valueRules []ValueReplaceRule, wrapRules []WrapRule, cdataRules []CdataRule, rawTags []string, deleteTags []string) *processor {
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

	deleteMap := make(map[string]bool)
	for _, tag := range deleteTags {
		deleteMap[tag] = true
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
		deleteTagMap:      deleteMap,
		elementStack:      make([]xml.StartElement, 0),
		skipDepth:         0,
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
			// スキップ中でなければ、他のトークンも書き出す
			if p.skipDepth == 0 {
				if err := p.encoder.EncodeToken(elem); err != nil {
					return fmt.Errorf("failed to encode token: %w", err)
				}
			}
		}
	}
	return p.encoder.Flush()
}

// handleStartElement は、開始タグを処理します。
func (p *processor) handleStartElement(se xml.StartElement) error {
	// 既にスキップ中か確認
	if p.skipDepth > 0 {
		p.skipDepth++ // スキップ対象のネストを深くする
		return nil    // 何も出力しない
	}

	// このタグが削除対象か確認
	if p.deleteTagMap[se.Name.Local] {
		p.skipDepth = 1 // スキップ開始
		return nil      // 何も出力しない
	}

	// --- 以下、スキップ対象でない場合の処理 ---

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
	// スキップ中なら何もしない
	if p.skipDepth > 0 {
		return nil
	}

	// --- 以下、スキップ対象でない場合の処理 ---

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
	// スキップ中か確認
	if p.skipDepth > 0 {
		p.skipDepth-- // スキップ対象のネストを浅くする
		return nil    // 何も出力しない
	}

	// --- 以下、スキップ対象でない場合の処理 ---

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
