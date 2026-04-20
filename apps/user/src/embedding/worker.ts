import { pipeline, env, type FeatureExtractionPipeline } from '@huggingface/transformers'

env.allowLocalModels = false
env.useBrowserCache = true

type InMessage =
  | { type: 'init'; modelId: string }
  | { type: 'embed'; jobId: string; text: string; prefix?: string }

type OutMessage =
  | { type: 'ready'; modelId: string; dims: number }
  | { type: 'progress'; status: string; loaded?: number; total?: number; file?: string }
  | { type: 'error'; message: string; modelId?: string; jobId?: string }
  | { type: 'embedded'; jobId: string; vec: number[]; dims: number }

let extractor: FeatureExtractionPipeline | null = null
let activeModel: string | null = null

const post = (m: OutMessage) => (self as unknown as Worker).postMessage(m)

async function init(modelId: string) {
  try {
    extractor = await pipeline('feature-extraction', modelId, {
      progress_callback: (p: { status: string; loaded?: number; total?: number; file?: string }) => {
        post({ type: 'progress', status: p.status, loaded: p.loaded, total: p.total, file: p.file })
      },
    })
    activeModel = modelId
    const probe = await extractor('hello', { pooling: 'mean', normalize: true })
    post({ type: 'ready', modelId, dims: probe.dims[probe.dims.length - 1] })
  } catch (err) {
    extractor = null
    activeModel = null
    post({ type: 'error', message: err instanceof Error ? err.message : 'init failed', modelId })
  }
}

async function embed(jobId: string, text: string, prefix?: string) {
  if (!extractor) {
    post({ type: 'error', message: 'worker not initialized', jobId })
    return
  }
  try {
    const input = prefix ? prefix + text : text
    const result = await extractor(input, { pooling: 'mean', normalize: true, truncation: true })
    const vec = Array.from(result.data as Float32Array)
    post({ type: 'embedded', jobId, vec, dims: vec.length })
  } catch (err) {
    post({ type: 'error', message: err instanceof Error ? err.message : 'embed failed', jobId })
  }
}

self.addEventListener('message', (e: MessageEvent<InMessage>) => {
  const msg = e.data
  if (msg.type === 'init') {
    if (activeModel === msg.modelId && extractor) {
      post({ type: 'ready', modelId: msg.modelId, dims: 0 })
      return
    }
    init(msg.modelId)
  } else if (msg.type === 'embed') {
    embed(msg.jobId, msg.text, msg.prefix)
  }
})
