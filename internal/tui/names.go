package tui

import (
	"math/rand"
	"strings"
)

// NetEase counts ASCII as 1 character and non-ASCII (Chinese, etc.) as 1.5
// characters. A nickname must be 3–12 characters.
const (
	nameMinWeight = 3.0
	nameMaxWeight = 12.0
)

var (
	nameChinesePrefixes = []string{
		"星海", "云端", "月光", "青柠", "方块", "像素", "晨曦", "松林",
		"薄荷", "暮色", "琥珀", "微风", "远山", "深蓝", "赤岩", "小鹿",
		"雾岛", "竹影", "海盐", "流萤",
	}
	nameChineseSuffixes = []string{
		"旅人", "矿工", "工匠", "探险家", "建筑师", "守望者", "向导", "船长",
		"钓手", "猎人", "园丁", "信使", "学徒", "骑士", "拾荒者", "幻想家",
	}
	nameEnglishAdjectives = []string{
		"Pixel", "Block", "Cube", "Sky", "Moon", "Cloud", "Frost", "Echo",
		"Nova", "Dock", "Mine", "Star", "Blue", "Ruby", "Neon", "Leaf",
		"Sand", "Iron",
	}
	nameEnglishNouns = []string{
		"Fox", "Wolf", "Owl", "Bee", "Cat", "Miner", "Scout", "Rider",
		"Pilot", "Smith", "Ranger", "Diver", "Keeper", "Maker", "Walker", "Otter",
	}
)

func nameWeight(value string) float64 {
	weight := 0.0
	for _, character := range strings.TrimSpace(value) {
		if character < 0x80 {
			weight += 1
		} else {
			weight += 1.5
		}
	}
	return weight
}

func validPlayerName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	weight := nameWeight(value)
	return weight >= nameMinWeight && weight <= nameMaxWeight
}

func randomChineseName() string {
	for attempt := 0; attempt < 80; attempt++ {
		name := nameChinesePrefixes[rand.Intn(len(nameChinesePrefixes))] +
			nameChineseSuffixes[rand.Intn(len(nameChineseSuffixes))]
		if validPlayerName(name) {
			return name
		}
	}
	return "方块旅人"
}

func randomEnglishName() string {
	for attempt := 0; attempt < 80; attempt++ {
		name := nameEnglishAdjectives[rand.Intn(len(nameEnglishAdjectives))] +
			nameEnglishNouns[rand.Intn(len(nameEnglishNouns))] +
			string(rune('0'+rand.Intn(10))) + string(rune('0'+rand.Intn(10)))
		if validPlayerName(name) {
			return name
		}
	}
	return "Cube" + string(rune('0'+rand.Intn(10))) + string(rune('0'+rand.Intn(10))) + string(rune('0'+rand.Intn(10)))
}

func nameCandidateButtons(names []string, kind pageHitKind) []customButton {
	buttons := make([]customButton, 0, len(names))
	for index, name := range names {
		buttons = append(buttons, customButton{label: name, kind: kind, index: index})
	}
	return buttons
}

func makeNameSuggestions(count int) (chinese, english []string) {
	seenChinese := map[string]bool{}
	seenEnglish := map[string]bool{}
	for len(chinese) < count {
		name := randomChineseName()
		if !seenChinese[name] {
			seenChinese[name] = true
			chinese = append(chinese, name)
		}
	}
	for len(english) < count {
		name := randomEnglishName()
		if !seenEnglish[name] {
			seenEnglish[name] = true
			english = append(english, name)
		}
	}
	return chinese, english
}
