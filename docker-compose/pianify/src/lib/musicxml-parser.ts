import type {
  Articulation,
  MeasureTiming,
  NoteEvent,
  ParsedScore,
  PartInfo,
  ScoreMetadata,
  TempoChange,
  TimeSignatureChange,
} from "./types";

function pitchToMidi(step: string, octave: number, alter: number): number {
  const stepToSemitone: Record<string, number> = {
    C: 0,
    D: 2,
    E: 4,
    F: 5,
    G: 7,
    A: 9,
    B: 11,
  };
  return (octave + 1) * 12 + (stepToSemitone[step] ?? 0) + alter;
}

function getTextContent(element: Element, tagName: string): string {
  const child = element.getElementsByTagName(tagName)[0];
  return child?.textContent?.trim() ?? "";
}

function getNumberContent(
  element: Element,
  tagName: string,
  fallback: number,
): number {
  const text = getTextContent(element, tagName);
  const value = Number.parseFloat(text);
  return Number.isNaN(value) ? fallback : value;
}

function parseParts(doc: Document): PartInfo[] {
  const partListElement = doc.getElementsByTagName("part-list")[0];
  if (!partListElement) return [];

  const scoreParts = partListElement.getElementsByTagName("score-part");
  const parts: PartInfo[] = [];
  for (let i = 0; i < scoreParts.length; i++) {
    const scorePart = scoreParts[i];
    parts.push({
      id: scorePart.getAttribute("id") ?? `P${i + 1}`,
      name: getTextContent(scorePart, "part-name") || `Part ${i + 1}`,
    });
  }
  return parts;
}

function parseTimeSignatures(doc: Document): TimeSignatureChange[] {
  const timeSignatures: TimeSignatureChange[] = [];
  const parts = doc.getElementsByTagName("part");
  if (parts.length === 0) return [{ measure: 1, beats: 4, beatType: 4 }];

  const firstPart = parts[0];
  const measures = firstPart.getElementsByTagName("measure");

  for (let m = 0; m < measures.length; m++) {
    const measure = measures[m];
    const attributes = measure.getElementsByTagName("attributes");
    for (let a = 0; a < attributes.length; a++) {
      const timeElements = attributes[a].getElementsByTagName("time");
      for (let t = 0; t < timeElements.length; t++) {
        const beats = getNumberContent(timeElements[t], "beats", 4);
        const beatType = getNumberContent(timeElements[t], "beat-type", 4);
        timeSignatures.push({ measure: m + 1, beats, beatType });
      }
    }
  }

  if (timeSignatures.length === 0) {
    return [{ measure: 1, beats: 4, beatType: 4 }];
  }
  return timeSignatures;
}

const DYNAMICS_VELOCITY: Record<string, number> = {
  ppp: 0.15,
  pp: 0.25,
  p: 0.4,
  mp: 0.55,
  mf: 0.7,
  f: 0.85,
  ff: 0.95,
  fff: 1.0,
};

function parseDynamicsVelocity(direction: Element): number | null {
  const dynamicsElements = direction.getElementsByTagName("dynamics");
  if (dynamicsElements.length === 0) return null;
  const dynamics = dynamicsElements[0];
  for (const [tag, velocity] of Object.entries(DYNAMICS_VELOCITY)) {
    if (dynamics.getElementsByTagName(tag).length > 0) return velocity;
  }
  return null;
}

function parseArticulation(note: Element): Articulation {
  const notations = note.getElementsByTagName("notations");
  if (notations.length === 0) return "none";
  const articulations = notations[0].getElementsByTagName("articulations");
  if (articulations.length === 0) return "none";
  const a = articulations[0];
  if (a.getElementsByTagName("staccatissimo").length > 0)
    return "staccatissimo";
  if (a.getElementsByTagName("staccato").length > 0) return "staccato";
  if (a.getElementsByTagName("accent").length > 0) return "accent";
  if (a.getElementsByTagName("tenuto").length > 0) return "tenuto";
  return "none";
}

function hasTieStop(note: Element): boolean {
  const tieElements = note.getElementsByTagName("tie");
  for (let t = 0; t < tieElements.length; t++) {
    if (tieElements[t].getAttribute("type") === "stop") return true;
  }
  return false;
}

function createNoteEvent(
  note: Element,
  partId: string,
  measureNumber: number,
  startMs: number,
  measureStartMs: number,
  durationBeats: number,
  durationMs: number,
  beatDurationMs: number,
  velocity: number,
): NoteEvent | null {
  const pitchElement = note.getElementsByTagName("pitch")[0];
  if (!pitchElement) return null;

  const step = getTextContent(pitchElement, "step");
  const octave = getNumberContent(pitchElement, "octave", 4);
  const alter = getNumberContent(pitchElement, "alter", 0);
  const voice = Number.parseInt(getTextContent(note, "voice") || "1", 10);
  const staff = Number.parseInt(getTextContent(note, "staff") || "1", 10);
  const articulation = parseArticulation(note);
  const noteVelocity =
    articulation === "accent" ? Math.min(velocity * 1.4, 1) : velocity;

  return {
    pitch: step,
    alter,
    octave,
    midiNote: pitchToMidi(step, octave, alter),
    partId,
    voice,
    staff,
    measure: measureNumber,
    beat: (startMs - measureStartMs) / beatDurationMs + 1,
    durationBeats,
    startMs,
    durationMs,
    tied: hasTieStop(note),
    articulation,
    velocity: noteVelocity,
  };
}

export function parseMusicXml(xmlString: string): ParsedScore {
  const parser = new DOMParser();
  const doc = parser.parseFromString(xmlString, "application/xml");

  const parserError = doc.getElementsByTagName("parsererror");
  if (parserError.length > 0) {
    throw new Error(`Invalid MusicXML: ${parserError[0].textContent}`);
  }

  const partInfos = parseParts(doc);
  const timeSignatures = parseTimeSignatures(doc);

  const title =
    getTextContent(doc.documentElement, "work-title") ||
    getTextContent(doc.documentElement, "movement-title") ||
    "Untitled";

  const identificationElement = doc.getElementsByTagName("identification")[0];
  let composer = "";
  if (identificationElement) {
    const creators = identificationElement.getElementsByTagName("creator");
    for (let c = 0; c < creators.length; c++) {
      if (creators[c].getAttribute("type") === "composer") {
        composer = creators[c].textContent?.trim() ?? "";
        break;
      }
    }
  }

  const tempoChanges: TempoChange[] = [];
  const measureTimings: MeasureTiming[] = [];
  const notes: NoteEvent[] = [];
  const xmlParts = doc.getElementsByTagName("part");
  let totalMeasures = 0;
  let maxEndMs = 0;

  for (let p = 0; p < xmlParts.length; p++) {
    const part = xmlParts[p];
    const partId = part.getAttribute("id") ?? `P${p + 1}`;
    const measures = part.getElementsByTagName("measure");
    totalMeasures = Math.max(totalMeasures, measures.length);

    let divisions = 1;
    let currentBpm = 120;
    let beatDurationMs = 60000 / currentBpm;
    let currentTimeMs = 0;
    let currentVelocity = 0.7;

    for (let m = 0; m < measures.length; m++) {
      const measure = measures[m];
      const measureNumber = m + 1;

      const attributes = measure.getElementsByTagName("attributes");
      for (let a = 0; a < attributes.length; a++) {
        const divisionsText = getTextContent(attributes[a], "divisions");
        if (divisionsText) {
          divisions = Number.parseInt(divisionsText, 10) || 1;
        }
      }

      const measureStartMs = currentTimeMs;

      if (p === 0) {
        measureTimings.push({
          measure: measureNumber,
          startMs: measureStartMs,
        });
      }

      const children = measure.children;

      for (let c = 0; c < children.length; c++) {
        const child = children[c];

        if (child.tagName === "direction") {
          const dynamicsVelocity = parseDynamicsVelocity(child);
          if (dynamicsVelocity !== null) {
            currentVelocity = dynamicsVelocity;
          }
          const sound = child.getElementsByTagName("sound")[0];
          if (sound?.getAttribute("tempo")) {
            const newBpm = Number.parseFloat(
              sound.getAttribute("tempo") as string,
            );
            if (!Number.isNaN(newBpm) && newBpm > 0) {
              currentBpm = newBpm;
              beatDurationMs = 60000 / currentBpm;
              if (p === 0) {
                tempoChanges.push({
                  measure: measureNumber,
                  beat: 1,
                  bpm: currentBpm,
                  startMs: currentTimeMs,
                });
              }
            }
          }
          continue;
        }

        if (child.tagName === "forward") {
          const dur = Number.parseInt(
            getTextContent(child, "duration") || "0",
            10,
          );
          currentTimeMs += (dur / divisions) * beatDurationMs;
          continue;
        }

        if (child.tagName === "backup") {
          const dur = Number.parseInt(
            getTextContent(child, "duration") || "0",
            10,
          );
          currentTimeMs -= (dur / divisions) * beatDurationMs;
          continue;
        }

        if (child.tagName !== "note") continue;

        const note = child;
        const isChord = note.getElementsByTagName("chord").length > 0;
        const isRest = note.getElementsByTagName("rest").length > 0;

        const durationDivisions = Number.parseInt(
          getTextContent(note, "duration") || "0",
          10,
        );
        const durationBeats = durationDivisions / divisions;
        const durationMs = durationBeats * beatDurationMs;

        const noteStartMs = isChord
          ? currentTimeMs - durationMs
          : currentTimeMs;

        if (!isRest) {
          const event = createNoteEvent(
            note,
            partId,
            measureNumber,
            noteStartMs,
            measureStartMs,
            durationBeats,
            durationMs,
            beatDurationMs,
            currentVelocity,
          );
          if (event) {
            notes.push(event);
            maxEndMs = Math.max(maxEndMs, noteStartMs + durationMs);
          }
        }

        if (!isChord) {
          currentTimeMs += durationMs;
        }
      }

      maxEndMs = Math.max(maxEndMs, currentTimeMs);
    }
  }

  if (tempoChanges.length === 0) {
    tempoChanges.push({ measure: 1, beat: 1, bpm: 120, startMs: 0 });
  }

  notes.sort((a, b) => a.startMs - b.startMs || a.midiNote - b.midiNote);

  const metadata: ScoreMetadata = {
    title,
    composer,
    parts: partInfos,
    timeSignatures,
    tempoChanges,
    measureTimings,
    totalDurationMs: maxEndMs,
    totalMeasures,
  };

  return { metadata, notes };
}
