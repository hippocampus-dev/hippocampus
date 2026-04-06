import { describe, expect, it } from "vitest";
import { parseMusicXml } from "../musicxml-parser";
import { filterNotesByHand, getActiveNotes } from "../score-queries";

const SIMPLE_SCORE = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="4.0">
  <work><work-title>Test</work-title></work>
  <identification>
    <creator type="composer">Tester</creator>
  </identification>
  <part-list>
    <score-part id="P1"><part-name>Piano</part-name></score-part>
  </part-list>
  <part id="P1">
    <measure number="1">
      <attributes>
        <divisions>1</divisions>
        <time><beats>4</beats><beat-type>4</beat-type></time>
        <staves>2</staves>
      </attributes>
      <direction>
        <direction-type><words/></direction-type>
        <sound tempo="120"/>
      </direction>
      <note>
        <pitch><step>C</step><octave>4</octave></pitch>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
      </note>
      <note>
        <pitch><step>E</step><octave>4</octave></pitch>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
      </note>
      <note>
        <rest/>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
      </note>
      <note>
        <pitch><step>G</step><octave>4</octave></pitch>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
      </note>
      <backup><duration>4</duration></backup>
      <note>
        <pitch><step>C</step><octave>3</octave></pitch>
        <duration>2</duration>
        <voice>2</voice>
        <type>half</type>
        <staff>2</staff>
      </note>
      <note>
        <pitch><step>G</step><octave>3</octave></pitch>
        <duration>2</duration>
        <voice>2</voice>
        <type>half</type>
        <staff>2</staff>
      </note>
    </measure>
  </part>
</score-partwise>`;

const CHORD_SCORE = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="4.0">
  <part-list>
    <score-part id="P1"><part-name>Piano</part-name></score-part>
  </part-list>
  <part id="P1">
    <measure number="1">
      <attributes>
        <divisions>1</divisions>
        <time><beats>4</beats><beat-type>4</beat-type></time>
      </attributes>
      <note>
        <pitch><step>C</step><octave>4</octave></pitch>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
      </note>
      <note>
        <chord/>
        <pitch><step>E</step><octave>4</octave></pitch>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
      </note>
      <note>
        <chord/>
        <pitch><step>G</step><octave>4</octave></pitch>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
      </note>
    </measure>
  </part>
</score-partwise>`;

const TIE_SCORE = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="4.0">
  <part-list>
    <score-part id="P1"><part-name>Piano</part-name></score-part>
  </part-list>
  <part id="P1">
    <measure number="1">
      <attributes>
        <divisions>1</divisions>
        <time><beats>4</beats><beat-type>4</beat-type></time>
      </attributes>
      <note>
        <pitch><step>C</step><octave>4</octave></pitch>
        <duration>2</duration>
        <voice>1</voice>
        <type>half</type>
        <staff>1</staff>
        <tie type="start"/>
      </note>
      <note>
        <pitch><step>C</step><octave>4</octave></pitch>
        <duration>2</duration>
        <voice>1</voice>
        <type>half</type>
        <staff>1</staff>
        <tie type="stop"/>
      </note>
    </measure>
  </part>
</score-partwise>`;

const ARTICULATION_SCORE = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="4.0">
  <part-list>
    <score-part id="P1"><part-name>Piano</part-name></score-part>
  </part-list>
  <part id="P1">
    <measure number="1">
      <attributes>
        <divisions>1</divisions>
        <time><beats>4</beats><beat-type>4</beat-type></time>
      </attributes>
      <direction>
        <direction-type><dynamics><f/></dynamics></direction-type>
        <sound tempo="120"/>
      </direction>
      <note>
        <pitch><step>C</step><octave>4</octave></pitch>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
        <notations><articulations><staccato/></articulations></notations>
      </note>
      <note>
        <pitch><step>D</step><octave>4</octave></pitch>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
        <notations><articulations><staccatissimo/></articulations></notations>
      </note>
      <note>
        <pitch><step>E</step><octave>4</octave></pitch>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
        <notations><articulations><accent/></articulations></notations>
      </note>
      <direction>
        <direction-type><dynamics><pp/></dynamics></direction-type>
      </direction>
      <note>
        <pitch><step>F</step><octave>4</octave></pitch>
        <duration>1</duration>
        <voice>1</voice>
        <type>quarter</type>
        <staff>1</staff>
      </note>
    </measure>
  </part>
</score-partwise>`;

describe("parseMusicXml", () => {
  it("parses metadata correctly", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    expect(result.metadata.title).toBe("Test");
    expect(result.metadata.composer).toBe("Tester");
    expect(result.metadata.parts).toHaveLength(1);
    expect(result.metadata.parts[0].id).toBe("P1");
    expect(result.metadata.parts[0].name).toBe("Piano");
  });

  it("parses time signature", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    expect(result.metadata.timeSignatures).toHaveLength(1);
    expect(result.metadata.timeSignatures[0].beats).toBe(4);
    expect(result.metadata.timeSignatures[0].beatType).toBe(4);
  });

  it("parses notes with correct pitches", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const pitchedNotes = result.notes.filter((n) => !n.rest);

    const rightHandNotes = pitchedNotes.filter((n) => n.staff === 1);
    expect(rightHandNotes).toHaveLength(3);
    expect(rightHandNotes[0].pitch).toBe("C");
    expect(rightHandNotes[0].octave).toBe(4);
    expect(rightHandNotes[1].pitch).toBe("E");
    expect(rightHandNotes[2].pitch).toBe("G");
  });

  it("calculates timing at 120 BPM", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const rightHandNotes = result.notes.filter((n) => !n.rest && n.staff === 1);

    expect(rightHandNotes[0].startMs).toBe(0);
    expect(rightHandNotes[0].durationMs).toBe(500);
    expect(rightHandNotes[1].startMs).toBe(500);
    expect(rightHandNotes[1].durationMs).toBe(500);
  });

  it("parses left hand notes", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const leftHandNotes = result.notes.filter((n) => !n.rest && n.staff === 2);

    expect(leftHandNotes).toHaveLength(2);
    expect(leftHandNotes[0].pitch).toBe("C");
    expect(leftHandNotes[0].octave).toBe(3);
    expect(leftHandNotes[0].durationMs).toBe(1000);
    expect(leftHandNotes[1].pitch).toBe("G");
    expect(leftHandNotes[1].octave).toBe(3);
  });

  it("parses chords with same start time", () => {
    const result = parseMusicXml(CHORD_SCORE);
    const notes = result.notes.filter((n) => !n.rest);

    expect(notes).toHaveLength(3);
    expect(notes[0].startMs).toBe(notes[1].startMs);
    expect(notes[1].startMs).toBe(notes[2].startMs);
    expect(notes[0].pitch).toBe("C");
    expect(notes[1].pitch).toBe("E");
    expect(notes[2].pitch).toBe("G");
  });

  it("detects ties", () => {
    const result = parseMusicXml(TIE_SCORE);
    const notes = result.notes.filter((n) => !n.rest);

    expect(notes).toHaveLength(2);
    expect(notes[0].tied).toBe(false);
    expect(notes[1].tied).toBe(true);
  });

  it("computes correct midi note numbers", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const c4 = result.notes.find(
      (n) => n.pitch === "C" && n.octave === 4 && n.staff === 1,
    );
    expect(c4?.midiNote).toBe(60);

    const g4 = result.notes.find(
      (n) => n.pitch === "G" && n.octave === 4 && n.staff === 1,
    );
    expect(g4?.midiNote).toBe(67);
  });

  it("parses staccato articulation", () => {
    const result = parseMusicXml(ARTICULATION_SCORE);
    const notes = result.notes;
    expect(notes[0].articulation).toBe("staccato");
  });

  it("parses staccatissimo articulation", () => {
    const result = parseMusicXml(ARTICULATION_SCORE);
    const notes = result.notes;
    expect(notes[1].articulation).toBe("staccatissimo");
  });

  it("parses accent articulation with boosted velocity", () => {
    const result = parseMusicXml(ARTICULATION_SCORE);
    const notes = result.notes;
    expect(notes[2].articulation).toBe("accent");
    expect(notes[2].velocity).toBeGreaterThan(notes[0].velocity);
  });

  it("defaults to none articulation", () => {
    const result = parseMusicXml(ARTICULATION_SCORE);
    const notes = result.notes;
    expect(notes[3].articulation).toBe("none");
  });

  it("parses dynamics velocity", () => {
    const result = parseMusicXml(ARTICULATION_SCORE);
    const notes = result.notes;
    expect(notes[0].velocity).toBeCloseTo(0.85, 2);
    expect(notes[3].velocity).toBeCloseTo(0.25, 2);
  });

  it("defaults velocity to mf when no dynamics", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const notes = result.notes.filter((n) => !n.rest);
    expect(notes[0].velocity).toBeCloseTo(0.7, 2);
  });
});

describe("filterNotesByHand", () => {
  it("returns all notes for 'both'", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const filtered = filterNotesByHand(result.notes, "both");
    expect(filtered).toEqual(result.notes);
  });

  it("returns only staff 1 for 'right'", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const filtered = filterNotesByHand(result.notes, "right");
    expect(filtered.every((n) => n.staff === 1)).toBe(true);
  });

  it("returns only staff 2 for 'left'", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const filtered = filterNotesByHand(result.notes, "left");
    expect(filtered.every((n) => n.staff === 2)).toBe(true);
  });
});

describe("getActiveNotes", () => {
  it("returns notes active at a given time", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const active = getActiveNotes(result.notes, 0);
    expect(active.length).toBeGreaterThan(0);
    expect(
      active.every((n) => n.startMs <= 0 && n.startMs + n.durationMs > 0),
    ).toBe(true);
  });

  it("returns empty array when no notes are active", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const active = getActiveNotes(result.notes, 999999);
    expect(active).toHaveLength(0);
  });

  it("returns correct notes at mid-measure", () => {
    const result = parseMusicXml(SIMPLE_SCORE);
    const active = getActiveNotes(result.notes, 250);
    const rightActive = active.filter((n) => n.staff === 1);
    expect(rightActive).toHaveLength(1);
    expect(rightActive[0].pitch).toBe("C");
  });
});
