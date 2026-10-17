import Link from "next/link";
import { LLMSettingsForm } from "@/features/settings/LLMSettingsForm";

export default function SettingsPage() {
  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-10">
      <Link href="/" className="text-sm text-muted hover:text-foreground">← Projetos</Link>
      <h1 className="mt-4 text-2xl font-semibold tracking-tight">Configurações</h1>
      <p className="mt-1 text-sm text-muted">Modelo e chave de acesso usados pelo agente.</p>
      <LLMSettingsForm />
    </main>
  );
}
