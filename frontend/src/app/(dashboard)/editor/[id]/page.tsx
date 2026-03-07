interface EditorPageProps {
  params: Promise<{ id: string }>;
}

export default async function EditorPage({ params }: EditorPageProps) {
  const { id } = await params;

  return (
    <div className="h-full flex items-center justify-center">
      <p className="text-sm text-muted-foreground">
        Edit strategy <span className="font-mono text-foreground">{id}</span> —
        implemented in Tasks 3.2.1 – 3.2.5
      </p>
    </div>
  );
}
