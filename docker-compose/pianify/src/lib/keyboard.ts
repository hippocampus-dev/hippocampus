export const PIANO_LOW = 21;
export const PIANO_HIGH = 108;

export function isBlackKey(midiNote: number): boolean {
  const n = midiNote % 12;
  return n === 1 || n === 3 || n === 6 || n === 8 || n === 10;
}

export interface KeyPosition {
  x: number;
  w: number;
}

export interface WhiteKey {
  midiNote: number;
}

export interface BlackKey {
  midiNote: number;
}

export const WHITE_KEYS: WhiteKey[] = [];
export const BLACK_KEYS: BlackKey[] = [];
export const KEY_POSITIONS: KeyPosition[] = [];

let whiteCount = 0;
for (let midi = PIANO_LOW; midi <= PIANO_HIGH; midi++) {
  if (!isBlackKey(midi)) whiteCount++;
}

const TOTAL_WHITE_KEYS = whiteCount;
const WHITE_KEY_UNIT = 1 / TOTAL_WHITE_KEYS;
const BLACK_KEY_UNIT = WHITE_KEY_UNIT * 0.6;

let whiteIndex = 0;
for (let midi = PIANO_LOW; midi <= PIANO_HIGH; midi++) {
  if (isBlackKey(midi)) {
    BLACK_KEYS.push({ midiNote: midi });
    KEY_POSITIONS[midi - PIANO_LOW] = {
      x: whiteIndex * WHITE_KEY_UNIT - BLACK_KEY_UNIT / 2,
      w: BLACK_KEY_UNIT,
    };
  } else {
    WHITE_KEYS.push({ midiNote: midi });
    KEY_POSITIONS[midi - PIANO_LOW] = {
      x: whiteIndex * WHITE_KEY_UNIT,
      w: WHITE_KEY_UNIT,
    };
    whiteIndex++;
  }
}

export interface HandColors {
  rightHand: string;
  rightHandDark: string;
  rightHandActive: string;
  rightHandDarkActive: string;
  leftHand: string;
  leftHandDark: string;
  leftHandActive: string;
  leftHandDarkActive: string;
}

const SOLFEGE: Record<string, string> = {
  C: "ド",
  D: "レ",
  E: "ミ",
  F: "ファ",
  G: "ソ",
  A: "ラ",
  B: "シ",
};

export function toSolfege(pitch: string, alter: number): string {
  const base = SOLFEGE[pitch] ?? pitch;
  if (alter === 1) return `${base}♯`;
  if (alter === -1) return `${base}♭`;
  return base;
}

export function resolveHandColors(element: Element): HandColors {
  const style = getComputedStyle(element);
  return {
    rightHand: style.getPropertyValue("--right-hand").trim(),
    rightHandDark: style.getPropertyValue("--right-hand-dark").trim(),
    rightHandActive: style.getPropertyValue("--right-hand-active").trim(),
    rightHandDarkActive: style
      .getPropertyValue("--right-hand-dark-active")
      .trim(),
    leftHand: style.getPropertyValue("--left-hand").trim(),
    leftHandDark: style.getPropertyValue("--left-hand-dark").trim(),
    leftHandActive: style.getPropertyValue("--left-hand-active").trim(),
    leftHandDarkActive: style
      .getPropertyValue("--left-hand-dark-active")
      .trim(),
  };
}
