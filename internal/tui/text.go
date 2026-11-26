package tui

import "strings"

// sanitizeTerminalText strips C0/C1 controls, bidi overrides and non-SGR escape
// sequences while preserving the SGR styling emitted by Lip Gloss. Dynamic names
// and messages can come from NetEase or the local operator; without this a
// crafted display name could emit terminal control sequences.
func sanitizeTerminalText(text string) string {
	if text == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(text))
	for index := 0; index < len(text); {
		character := text[index]
		if character == 0x1b {
			if index+1 < len(text) && text[index+1] == '[' {
				end := index + 2
				for end < len(text) && !(text[end] >= '@' && text[end] <= '~') {
					end++
				}
				if end < len(text) {
					if text[end] == 'm' {
						builder.WriteString(text[index : end+1])
					}
					index = end + 1
					continue
				}
			}
			if index+1 < len(text) && text[index+1] == ']' {
				// OSC (for example clipboard/title) - drop through BEL or ST.
				end := index + 2
				for end < len(text) && text[end] != 0x07 {
					if text[end] == 0x1b && end+1 < len(text) && text[end+1] == '\\' {
						end += 2
						break
					}
					end++
				}
				if end < len(text) && text[end] == 0x07 {
					end++
				}
				index = end
				continue
			}
			if index+1 < len(text) {
				index += 2
			} else {
				index++
			}
			continue
		}
		// Only strip ASCII controls here; C1 controls are UTF-8 runes and are
		// handled in stripFormatRunes, so Chinese bytes are never cut.
		if character < 0x20 || character == 0x7f {
			index++
			continue
		}
		builder.WriteByte(character)
		index++
	}
	return stripFormatRunes(builder.String())
}

// sanitizeMultilineText is for pasted/user text: it keeps newlines for text
// areas but removes every escape and other control character.
func sanitizeMultilineText(text string) string {
	if text == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(text))
	for index := 0; index < len(text); {
		character := text[index]
		switch {
		case character == '\n':
			builder.WriteByte('\n')
			index++
		case character == '\r':
			index++
		case character == 0x1b:
			index++
			if index < len(text) && text[index] == '[' {
				for index < len(text) && !(text[index] >= '@' && text[index] <= '~') {
					index++
				}
				if index < len(text) {
					index++
				}
			} else if index < len(text) && text[index] == ']' {
				index++
				for index < len(text) && text[index] != 0x07 {
					if text[index] == 0x1b && index+1 < len(text) && text[index+1] == '\\' {
						index += 2
						break
					}
					index++
				}
				if index < len(text) && text[index] == 0x07 {
					index++
				}
			} else if index < len(text) {
				index++
			}
		case character < 0x20 || character == 0x7f:
			index++
		default:
			builder.WriteByte(character)
			index++
		}
	}
	return stripFormatRunes(builder.String())
}

func stripFormatRunes(text string) string {
	return strings.Map(func(character rune) rune {
		switch {
		case character >= 0x80 && character <= 0x9f:
			return -1
		case character >= 0x202a && character <= 0x202e:
			return -1
		case character >= 0x2066 && character <= 0x2069:
			return -1
		default:
			return character
		}
	}, text)
}
