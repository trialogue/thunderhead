    "use client";

    import { useEffect, useState } from "react";
    import Demo from "@/components/ui/Hero/demo/Demo";
    import { FiGithub } from "react-icons/fi";
    import { ScrollText } from "lucide-react";
    import { Button } from "@/components/ui/Buttons";
    import DarkVeil  from "@/components/ui/Background";

    const GITHUB_URL = "https://github.com/trialogue/thunderhead";
    const DOCS_URL = "/docs";

    export default function Hero() {
    const [mounted, setMounted] = useState(false);

    useEffect(() => {
        const t = setTimeout(() => setMounted(true), 80);
        return () => clearTimeout(t);
    }, []);

    const fade = (delay: number): React.CSSProperties => ({
        opacity: mounted ? 1 : 0,
        transform: mounted ? "translateY(0)" : "translateY(14px)",
        transition: "opacity 0.7s ease, transform 0.7s ease",
        transitionDelay: `${delay}ms`,
    });

    return (
        <section className="relative isolate overflow-hidden min-h-dvh">
        <div className="absolute inset-x-0 -z-10 pointer-events-none">
            <div style={{ width: '100%', height: '100dvh', position: 'relative' }}>
            <DarkVeil
            hueShift={0}
            noiseIntensity={0}
            scanlineIntensity={0}
            speed={0.5}
            scanlineFrequency={0}
            warpAmount={0}
            />
            </div>
        </div>

        <div className="relative z-10 mx-auto flex w-full max-w-6xl flex-col gap-16 pt-28 pb-12 px-6 md:px-10">
        <div className="flex flex-col items-start gap-6 w-full">
            <h1
                className="max-w-2xl text-2xl font-medium tracking-tight text-zinc-200 md:text-3xl md:text-balance"
                style={fade(80)}
            >
               Forecast the intent of every HTTP request. <br />
               Score silently. Respond proportionally.
            </h1>

          <div style={fade(240)} className="flex gap-2">
            <Button variant="default" size="default" asChild>
              <a href={GITHUB_URL} target="_blank" rel="noopener noreferrer">
                <FiGithub className="size-4" />
                View on Github
              </a>
            </Button>

            <Button variant="default" size="default" asChild>
              <a href={DOCS_URL}>
                <ScrollText className="size-4" />
                Docs
              </a>
            </Button>
          </div>
        </div>
      </div>

        {/* Dashboard demo - static image on mobile, live demo on desktop */}
        <div
        className="relative z-10 mx-auto w-full max-w-6xl"
        style={fade(360)}
        >
        <div className="md:hidden relative h-full ml-2">
        <img
            src="/thunderhead_dashboard.png"
            alt="Thunderhead dashboard"
            className="w-full h-full object-cover rounded-l-xl"
        />
        </div>

            {/* Desktop: live interactive demo */}
            <div className="hidden md:block relative rounded-l-xl md:rounded-xl overflow-hidden">
                <div className="bg-white/5 backdrop-blur-xl border border-white/10 shadow-[0_8px_32px_rgba(0,0,0,0.25)] rounded-2xl p-2">            
                    <Demo />
                </div>
            </div>
        </div>
        </section>
    );
    }