import { Suspense } from "react";
import { Workspace } from "@/features/agent/Workspace";

async function ProjectWorkspace({ params }: Pick<PageProps<"/projects/[id]">, "params">) {
  const { id } = await params;
  return <Workspace projectId={id} />;
}

export default function ProjectPage(props: PageProps<"/projects/[id]">) {
  return (
    <Suspense fallback={<p className="p-8 text-muted">Abrindo projeto…</p>}>
      <ProjectWorkspace params={props.params} />
    </Suspense>
  );
}
