import { NewGameForm } from "@/features/projects/NewGameForm";
import { ProjectList } from "@/features/projects/ProjectList";

export default function Home() {
  return (
    <main className="mx-auto w-full max-w-3xl px-4 py-10 sm:py-16">
      <header className="mb-8">
        <h1 className="text-3xl font-semibold tracking-tight">
          Game <span className="text-accent">Forge</span>
        </h1>
        <p className="mt-2 text-muted">Descreva um jogo mobile. O agente projeta, programa em Go + Ebitengine e testa.</p>
      </header>
      <NewGameForm />
      <h2 className="mt-12 mb-3 text-sm font-medium uppercase tracking-wide text-muted">Projetos</h2>
      <ProjectList />
    </main>
  );
}
