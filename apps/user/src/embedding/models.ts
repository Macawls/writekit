export interface EmbeddingModel {
  id: string
  label: string
  dims: number
  approxSizeMB: number
  docPrefix?: string
  description: string
  recommended?: boolean
}

export const MODELS: EmbeddingModel[] = [
  {
    id: 'mixedbread-ai/mxbai-embed-xsmall-v1',
    label: 'mxbai xsmall (fast)',
    dims: 384,
    approxSizeMB: 24,
    description: 'Tiny, fast download. Retrieval-tuned, English. Good first choice to try things out.',
  },
  {
    id: 'onnx-community/embeddinggemma-300m-ONNX',
    label: 'EmbeddingGemma 300M (recommended)',
    dims: 768,
    approxSizeMB: 200,
    description: 'Google\u2019s on-device embedding model. Multilingual (100+ langs), 2K context, top of the under-500M MTEB leaderboard.',
    recommended: true,
  },
  {
    id: 'nomic-ai/nomic-embed-text-v1.5',
    label: 'Nomic Embed v1.5 (long context)',
    dims: 768,
    approxSizeMB: 137,
    docPrefix: 'search_document: ',
    description: 'Strong on long passages. Larger download.',
  },
  {
    id: 'Xenova/all-MiniLM-L6-v2',
    label: 'MiniLM-L6 (legacy baseline)',
    dims: 384,
    approxSizeMB: 23,
    description: 'Old reliable. Kept for comparison; mxbai xsmall is generally better at the same size.',
  },
]

export const DEFAULT_MODEL_ID = 'mixedbread-ai/mxbai-embed-xsmall-v1'

export function findModel(id: string): EmbeddingModel | undefined {
  return MODELS.find(m => m.id === id)
}
