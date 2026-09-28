import { Button } from "@/components/ui/Buttons";

const GITHUB_URL = "https://github.com/thunderheadhq/thunderhead";

export default function DocsPage() {
  return (
    <section className="bg-foreground relative flex min-h-screen flex-col items-center justify-center gap-6 px-6 text-center">

      <a href="/" className="animate-fade-up delay-1 text-sm text-muted-foreground hover:text-white transition-colors">
        ← Go back
      </a>

      <h1 className="animate-fade-up delay-1 text-4xl md:text-5xl">
        Docs are coming soon.
      </h1>

      <p className="max-w-md animate-fade-up delay-2">
        Thunderhead is still early. In the meantime, the README covers setup,
        config, and the scoring model.
      </p>

      <div className="animate-fade-up delay-3">
        <Button variant="outline" size="default" asChild>
          <a href={GITHUB_URL} target="_blank" rel="noopener noreferrer">
            Read the README
          </a>
        </Button>
      </div>
    </section>
  );
}