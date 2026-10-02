"use client";

import { useState } from "react";
import {
  Button,
} from "@/components/ui/Buttons";
import { ScrollText } from 'lucide-react';

const bar = "backdrop-blur-xl bg-white/5";

const Logo = ({ onClick }: { onClick?: () => void }) => (
  <a href="#top" onClick={onClick} className="flex items-center gap-2 shrink-0">
    <img src="/favicon-192.png" alt="thunderhead" className="h-6 w-6" />
    <span className="text-sm font-medium text-zinc-200 tracking-tight">
      thunderhead 
    </span>
  </a>
);

export default function Navbar() {
  const [menuOpen, setMenuOpen] = useState(false);

  const scrollTop = () => {
    window.scrollTo({ top: 0, behavior: "smooth" });
    setMenuOpen(false);
  };

  return (
    <nav className="fixed inset-x-0 top-0 z-50 w-full flex justify-center px-4 pt-4">

      {/* Desktop */}
      <div
        className={`hidden sm:flex items-center justify-between px-4 py-3 rounded-xl w-full max-w-3xl ${bar}`}
      >
        <Logo /> 

        <div className="flex items-center gap-2">
          <Button variant="ghost" size="icon" asChild>
            <a href="/docs"><ScrollText /></a> 
          </Button>
          <Button variant="secondary" size="sm" asChild>
            <a href="#quickstart">Install</a>
          </Button>
        </div>
      </div>

      {/* Mobile */}
      <div className="sm:hidden flex flex-col w-full">
        <div
          className={`flex items-center justify-between px-4 py-2 w-full rounded-xl shadow-lg shadow-black/20 ${bar}`}
        >
          <Logo onClick={scrollTop} />
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="sm" asChild>
                <a href="/docs"><ScrollText /></a>
            </Button>
            <Button variant="default" size="sm" asChild>
              <a href="#quickstart">Install</a>
            </Button>
          </div>
        </div>
      </div>
    </nav>
  );
}