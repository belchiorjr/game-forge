export function PreviewPane() {
  return (
    <div className="flex h-full min-h-[320px] items-center justify-center p-6">
      <div className="flex aspect-[9/16] h-full max-h-[640px] flex-col items-center justify-center rounded-[2rem] border-8 border-border bg-surface p-6 text-center text-sm text-muted">
        <p className="font-medium text-foreground">Preview WASM</p>
        <p className="mt-2">Chega no passo 4 do MVP: o template Ebitengine compilado para WebAssembly roda aqui.</p>
      </div>
    </div>
  );
}
