const STEPS = [
  {
    n: "1",
    title: "Install",
    body: "Install the binary directly with Go. No external dependencies required.",
    code: "go install github.com/trialogue/thunderhead/cmd/thunderhead@latest",
  },
  {
    n: "2",
    title: "Configure",
    body: "Point Thunderhead at your upstream and tune your scoring thresholds in config.json.",
  },
  {
    n: "3",
    title: "Run",
    body: "Start Thunderhead in front of your app. Bots get scored, tarpitted, or blocked.",
    code: "thunderhead -config config.json",
  },
];

export default function Steps({ inView }: { inView: boolean }) {
  return (
    <div className="flex flex-col">
      {STEPS.map((step, i) => (
        <div
          key={step.n}
          className={`flex gap-4 py-6 min-w-0`}
          style={{
            opacity: inView ? 1 : 0,
            transform: inView ? "translateX(0)" : "translateX(-12px)",
            transition: `opacity 500ms ease ${i * 100}ms, transform 500ms cubic-bezier(0.16,1,0.3,1) ${i * 100}ms`,
          }}
        >
          <span className="text-sm text-white/20 shrink-0">{step.n}</span>
          <div className="flex flex-col gap-1 min-w-0 w-full">
            <span className="text-sm text-white tracking-tight">{step.title}</span>
            <span className="text-sm text-white/60 leading-relaxed">{step.body}</span>
            {step.code && (
              <div className="mt-1 bg-zinc-900 rounded-md px-3 py-2 text-xs text-white/60 overflow-x-auto whitespace-nowrap w-full">
                <span className="text-white/30">$ </span>{step.code}
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}