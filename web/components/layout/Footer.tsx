"use client";

export default function Footer() {
  const projectLinks = [
    { label: "Github",  href: "https://github.com/thunderheadhq/thunderhead" },
    { label: "License", href: "https://github.com/thunderheadhq/thunderhead/blob/main/LICENSE" },
    { label: "Issues",  href: "https://github.com/thunderheadhq/thunderhead/issues" },
  ];

  const docLinks = [
    { label: "Quickstart",  href: "/docs/quickstart"  },
    { label: "Methodology", href: "/docs" },
    { label: "Features",    href: "/docs/api"    },
  ];

  return (
    <footer className="px-8 py-16 bg-white/5">
      <div className="mx-auto max-w-6xl">

        {/* Top row */}
        <div className="mb-12 flex flex-wrap justify-between gap-12">

          {/* Brand */}
          <div className="max-w-sm">
            <div className="mb-4 text-white flex flex-row gap-2 items-end">
              <img src="/reaper.png" alt="Thunderhead" className="h-8 w-8" />
              Thunderhead
            </div>
            <p className="m-0 max-w-lg text-sm text-zinc-500">
              Passive intent-scoring reverse proxy. Silently watches, scores,
              and mitigates bot traffic, no CAPTCHAs, no challenges, just behavior.
            </p>
          </div>

          {/* Links */}
          <div className="flex flex-wrap gap-12">
            {[{ title: "Project", links: projectLinks }, { title: "Docs", links: docLinks }].map(({ title, links }) => (
              <div key={title}>
                <div className="mb-4 text-xs text-zinc-500">
                  {title}
                </div>
                <div className="flex flex-col gap-3">
                  {links.map((link) => (
                    <a
                      key={link.label}
                      href={link.href}
                      target={link.href.startsWith("http") ? "_blank" : undefined}
                      className="text-sm text-zinc-500 no-underline transition-colors duration-150 hover:text-white"
                    >
                      {link.label}
                    </a>
                  ))}
                </div>
              </div>
            ))}
          </div>

        </div>

        {/* Bottom row */}
        <div className="flex flex-wrap items-center justify-between gap-4">
          <span className="text-xs text-zinc-600">
            © 2026 Bhavdeep Arora · MIT License
          </span>
        </div>

      </div>
    </footer>
  );
}