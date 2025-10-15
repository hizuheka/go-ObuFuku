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

	elementStack []xml.StartElement
}

// newProcessor は、新しいprocessorを初期化します。
func newProcessor(r io.Reader, w io.Writer, nameRules []NameReplaceRule, insertRules []InsertBeforeRule, valueRules []ValueReplaceRule) *processor {
	decoder := xml.NewDecoder(r)
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")

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
	if len(strings.TrimSpace(string(cd))) == 0 {
		return p.encoder.EncodeToken(cd)
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
		return fmt.Errorf("invalid XML structure: found end element without matching start element")
	}

	lastStartedElem := p.elementStack[len(p.elementStack)-1]
	p.elementStack = p.elementStack[:len(p.elementStack)-1]
	if err := p.encoder.EncodeToken(xml.EndElement{Name: lastStartedElem.Name}); err != nil {
		return fmt.Errorf("failed to encode end element: %w", err)
	}
	return nil
}
