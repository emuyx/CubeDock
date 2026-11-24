// NetEase counts ASCII as 1 character and non-ASCII (Chinese, etc.) as 1.5
// characters. A nickname must be 3–12 characters.
const MIN_WEIGHT = 3;
const MAX_WEIGHT = 12;

const chinesePrefixes = [
  '星海', '云端', '月光', '青柠', '方块', '像素', '晨曦', '松林',
  '薄荷', '暮色', '琥珀', '微风', '远山', '深蓝', '赤岩', '小鹿',
  '雾岛', '竹影', '海盐', '流萤',
];
const chineseSuffixes = [
  '旅人', '矿工', '工匠', '探险家', '建筑师', '守望者', '向导', '船长',
  '钓手', '猎人', '园丁', '信使', '学徒', '骑士', '拾荒者', '幻想家',
];
const englishAdjectives = [
  'Pixel', 'Block', 'Cube', 'Sky', 'Moon', 'Cloud', 'Frost', 'Echo',
  'Nova', 'Dock', 'Mine', 'Star', 'Blue', 'Ruby', 'Neon', 'Leaf',
  'Sand', 'Iron',
];
const englishNouns = [
  'Fox', 'Wolf', 'Owl', 'Bee', 'Cat', 'Miner', 'Scout', 'Rider',
  'Pilot', 'Smith', 'Ranger', 'Diver', 'Keeper', 'Maker', 'Walker', 'Otter',
];

function pick(list) {
  return list[Math.floor(Math.random() * list.length)];
}

function randomInt(min, max) {
  return min + Math.floor(Math.random() * (max - min + 1));
}

function hasControlCharacter(value) {
  return [...value].some(character => {
    const code = character.codePointAt(0);
    return code < 0x20 || code === 0x7f;
  });
}

export function nameWeight(name) {
  let weight = 0;
  for (const character of name.trim()) {
    weight += character.codePointAt(0) < 0x80 ? 1 : 1.5;
  }
  return weight;
}

export function isValidName(name) {
  const value = name.trim();
  if (!value || hasControlCharacter(value)) return false;
  const weight = nameWeight(value);
  return weight >= MIN_WEIGHT && weight <= MAX_WEIGHT;
}

export function randomChineseName() {
  for (let attempt = 0; attempt < 80; attempt += 1) {
    const name = `${pick(chinesePrefixes)}${pick(chineseSuffixes)}`;
    if (isValidName(name)) return name;
  }
  return '方块旅人';
}

export function randomEnglishName() {
  for (let attempt = 0; attempt < 80; attempt += 1) {
    const name = `${pick(englishAdjectives)}${pick(englishNouns)}${randomInt(10, 99)}`;
    if (isValidName(name)) return name;
  }
  return `Cube${randomInt(100, 999)}`;
}

export function makeNameSuggestions(count = 3) {
  const unique = list => [...new Set(list)];
  return {
    chinese: unique(Array.from({ length: count * 3 }, randomChineseName)).slice(0, count),
    english: unique(Array.from({ length: count * 3 }, randomEnglishName)).slice(0, count),
  };
}

export { MIN_WEIGHT, MAX_WEIGHT };
