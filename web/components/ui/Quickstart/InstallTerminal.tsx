"use client";

import { useEffect, useState } from "react";

const LINES = [
  { text: "$ go install github.com/thunderheadhq/thunderhead/cmd/thunderhead@latest", color: "text-white/70", delay: 0 },
  { text: "go: downloading github.com/thunderheadhq/thunderhead v0.1.1", color: "text-white/25", delay: 800 },
  { text: "go: downloading github.com/charmbracelet/bubbletea v1.3.10", color: "text-white/25", delay: 1200 },
  { text: "go: downloading github.com/charmbracelet/lipgloss v1.1.0", color: "text-white/25", delay: 1600 },
  { text: "", color: "", delay: 2000 },
  { text: "$ thunderhead -config config.json", color: "text-white/70", delay: 2400 },
  { text: "2026/05/23 thunderhead starting on :8080 -> http://localhost:3000", color: "text-white/30", delay: 2900 },
  { text: "2026/05/23 thresholds: tarpit=40 block=75", color: "text-white/30", delay: 3100 },
  { text: "2026/05/23 loaded 4 disallowed paths from robots.txt", color: "text-white/30", delay: 3300 },
  { text: "", color: "", delay: 3600 },
  { text: `{"ip":"185.220.101.5","path":"/wp-login.php","score":95,"action":"block"}`, color: "text-red-400", delay: 4000 },
  { text: `{"ip":"64.233.160.0","path":"/","score":8,"action":"allow"}`, color: "text-green-400", delay: 4600 },
  { text: `{"ip":"103.21.244.0","path":"/page34","score":54,"action":"tarpit"}`, color: "text-amber-400", delay: 5200 },
];

export default function InstallTerminal({ animate }: { animate: boolean }) {
  const [visibleCount, setVisibleCount] = useState(0);

  useEffect(() => {
    if (!animate) return;
    setVisibleCount(0);
    const timeouts = LINES.map((line, i) =>
      setTimeout(() => setVisibleCount(i + 1), line.delay)
    );
    return () => timeouts.forEach(clearTimeout);
  }, [animate]);

  return (
    <div className="bg-zinc-900 rounded-xl overflow-hidden sm:mt-8">
      {/* Title bar */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-white/10 bg-white/5">
        <div className="flex gap-1.5">
          {["#ff5f57", "#febc2e", "#28c840"].map((c) => (
            <div key={c} style={{ background: c }} className="w-3 h-3 rounded-full" />
          ))}
        </div>
        <span className="text-sm text-white/70">
          thunderhead - install & run
        </span>
        <div className="w-10" />
      </div>

      {/* Body */}
      <div className="p-5 flex flex-col min-h-72 bg-black/50 overflow-x-auto">
        {LINES.slice(0, visibleCount).map((line, i) => (
          <div key={i} className={`text-xs leading-relaxed whitespace-pre overflow-hidden ${line.color}`}>
            {line.text || "\u00A0"}
          </div>
        ))}
        {visibleCount < LINES.length && visibleCount > 0 && (
          <div className="inline-block w-2 h-4 bg-blue-400 opacity-80" />
        )}
      </div>
    </div>
  );
}