export interface PasteMetadata {
  id: string;
  language: string;
  title: string;
  createdAt: string;
  expiresAt: string | null;
  size: number;
}

export interface PasteWithContent extends PasteMetadata {
  content: string;
}

export interface PasteCreateInput {
  content: string;
  language: string;
  title: string;
  expiresAt: Date | null;
}

export interface PasteRepository {
  create(input: PasteCreateInput): Promise<PasteMetadata>;
  findById(id: string): Promise<PasteWithContent | null>;
  deleteById(id: string): Promise<void>;
}

export interface ExplanationRepository {
  get(pasteId: string): Promise<string | null>;
  set(pasteId: string, explanation: string, ttlSeconds?: number): Promise<void>;
}
