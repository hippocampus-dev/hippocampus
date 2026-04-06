import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useMemo, useState } from "react";
import FallingNotes from "@/components/FallingNotes";
import PianoKeyboard from "@/components/PianoKeyboard";
import PlaybackControls from "@/components/PlaybackControls";
import SheetMusic from "@/components/SheetMusic";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { usePlayback } from "@/hooks/use-playback";
import { parseMusicXml } from "@/lib/musicxml-parser";
import type { ParsedScore } from "@/lib/types";
import { fetchScoreList, fetchScoreXml } from "@/server/functions";

export const Route = createFileRoute("/")({
  loader: () => fetchScoreList(),
  component: Player,
  errorComponent: ({ error }) => (
    <div className="min-h-screen bg-background flex items-center justify-center">
      <p className="text-sm text-destructive">
        Failed to load scores:{" "}
        {error instanceof Error ? error.message : "Unknown error"}
      </p>
    </div>
  ),
});

function Player() {
  const scores = Route.useLoaderData();
  const [selectedFile, setSelectedFile] = useState("");
  const [uploadedScore, setUploadedScore] = useState<{
    filename: string;
    musicXml: string;
    score: ParsedScore;
  } | null>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);

  const scoreQuery = useQuery({
    queryKey: ["score", selectedFile],
    queryFn: () => fetchScoreXml({ data: { filename: selectedFile } }),
    enabled: selectedFile !== "" && uploadedScore === null,
  });

  const parsedScore = useMemo(() => {
    if (uploadedScore) return uploadedScore;
    if (!scoreQuery.data) return null;
    try {
      const score = parseMusicXml(scoreQuery.data);
      return { filename: selectedFile, musicXml: scoreQuery.data, score };
    } catch {
      return null;
    }
  }, [uploadedScore, scoreQuery.data, selectedFile]);

  const score = parsedScore?.score ?? null;

  const {
    status,
    currentTimeMs,
    tempoMultiplier,
    handSelection,
    activeNotes,
    filteredNotes,
    volume,
    muted,
    checkpointMs,
    play,
    pause,
    stop,
    seek,
    setTempoMultiplier,
    setHandSelection,
    setVolume,
    setMuted,
    setCheckpoint,
    clearCheckpoint,
    playFromCheckpoint,
  } = usePlayback({
    notes: score?.notes ?? [],
    totalDurationMs: score?.metadata.totalDurationMs ?? 0,
  });

  const handleFileSelect = useCallback((filename: string) => {
    setUploadedScore(null);
    setSelectedFile(filename);
  }, []);

  const handleFileUpload = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      if (!file) return;

      const reader = new FileReader();
      reader.onload = (e) => {
        const xml = e.target?.result as string;
        try {
          const parsed = parseMusicXml(xml);
          setUploadedScore({
            filename: file.name,
            musicXml: xml,
            score: parsed,
          });
          setUploadError(null);
          setSelectedFile("");
        } catch (error) {
          setUploadedScore(null);
          setUploadError(
            error instanceof Error ? error.message : "Failed to parse MusicXML",
          );
        }
      };
      reader.readAsText(file);
    },
    [],
  );

  const displayFile = uploadedScore?.filename ?? selectedFile;

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b border-border px-3 py-3 sm:px-6 sm:py-4">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <h1 className="text-xl font-bold">Pianify</h1>
          <div className="flex items-center gap-2 sm:gap-4">
            {scores.length > 0 && (
              <Select value={selectedFile} onValueChange={handleFileSelect}>
                <SelectTrigger size="sm" className="flex-1 sm:flex-initial">
                  <SelectValue placeholder="Select a score..." />
                </SelectTrigger>
                <SelectContent>
                  {scores.map((filename) => (
                    <SelectItem key={filename} value={filename}>
                      {filename.replace(/\.(musicxml|xml)$/, "")}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
            <label className="text-sm text-muted-foreground cursor-pointer hover:text-foreground border border-input rounded-md px-3 py-1.5 whitespace-nowrap">
              Upload MusicXML
              <input
                type="file"
                accept=".musicxml,.xml"
                onChange={handleFileUpload}
                className="hidden"
              />
            </label>
          </div>
        </div>
      </header>

      <main className="px-3 py-3 space-y-4 sm:px-6 sm:py-6">
        {(scoreQuery.isError || uploadError !== null) && (
          <div className="p-3 bg-destructive/10 text-destructive rounded-md text-sm">
            {uploadError ??
              (scoreQuery.error instanceof Error
                ? scoreQuery.error.message
                : "Failed to load score")}
          </div>
        )}

        {parsedScore !== null && (
          <>
            <div className="text-center">
              <h2 className="text-lg font-semibold">
                {parsedScore.score.metadata.title}
              </h2>
              {parsedScore.score.metadata.composer && (
                <p className="text-sm text-muted-foreground">
                  {parsedScore.score.metadata.composer}
                </p>
              )}
            </div>

            <SheetMusic
              musicXml={parsedScore.musicXml}
              currentTimeMs={currentTimeMs}
              totalDurationMs={parsedScore.score.metadata.totalDurationMs}
              totalMeasures={parsedScore.score.metadata.totalMeasures}
            />

            <div>
              <FallingNotes
                notes={filteredNotes}
                currentTimeMs={currentTimeMs}
              />
              <PianoKeyboard activeNotes={activeNotes} />
            </div>

            <PlaybackControls
              status={status}
              currentTimeMs={currentTimeMs}
              totalDurationMs={parsedScore.score.metadata.totalDurationMs}
              tempoMultiplier={tempoMultiplier}
              handSelection={handSelection}
              onPlay={play}
              onPause={pause}
              onStop={stop}
              onSeek={seek}
              onTempoChange={setTempoMultiplier}
              onHandChange={setHandSelection}
              volume={volume}
              muted={muted}
              checkpointMs={checkpointMs}
              onVolumeChange={setVolume}
              onMuteToggle={setMuted}
              onSetCheckpoint={setCheckpoint}
              onClearCheckpoint={clearCheckpoint}
              onPlayFromCheckpoint={playFromCheckpoint}
            />
          </>
        )}

        {!displayFile && !scoreQuery.isLoading && (
          <div className="flex items-center justify-center h-96 text-muted-foreground">
            <p>Select or upload a MusicXML file to get started.</p>
          </div>
        )}

        {scoreQuery.isLoading && (
          <div className="flex items-center justify-center h-96 text-muted-foreground">
            <p>Loading...</p>
          </div>
        )}
      </main>
    </div>
  );
}
