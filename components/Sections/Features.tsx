import React from 'react';
import BrutalCard from '../ui/BrutalCard';
import { Network, Key, Layers, FileCode, Terminal, Database } from 'lucide-react';

const Features: React.FC = () => {
  return (
    <section id="how-it-works" className="py-12 md:py-20 bg-brand-yellow/10 border-y-2 border-brand-black">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="mb-10 md:mb-16 text-center">
          <h2 className="text-3xl md:text-4xl font-bold mb-4 md:mb-6">How It Works</h2>
          <p className="text-lg md:text-xl max-w-2xl mx-auto">This server acts as a <span className="font-bold underline decoration-brand-yellow decoration-4">smart bridge</span> between an AI Agent and your remote infrastructure.</p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 md:gap-8">
          <BrutalCard className="hover:translate-y-[-4px] transition-transform duration-300">
            <div className="w-12 h-12 bg-brand-yellow border-2 border-brand-black rounded-lg flex items-center justify-center mb-6 shadow-brutal-sm">
              <Network size={24} />
            </div>
            <h3 className="text-xl font-bold mb-3">Direct SSH Bridge</h3>
            <p className="text-gray-600 leading-relaxed text-sm md:text-base">
              The agent doesn't need SSH libraries. It calls simple tools, and the server handles connections and relaying.
            </p>
          </BrutalCard>

          <BrutalCard className="hover:translate-y-[-4px] transition-transform duration-300">
            <div className="w-12 h-12 bg-blue-300 border-2 border-brand-black rounded-lg flex items-center justify-center mb-6 shadow-brutal-sm">
              <Key size={24} />
            </div>
            <h3 className="text-xl font-bold mb-3">Managed Identity</h3>
            <p className="text-gray-600 leading-relaxed text-sm md:text-base">
              Auto-generated Ed25519 keys. Call <code>identity()</code> to get the public key, add it to your host, and connect passwordless.
            </p>
          </BrutalCard>

          <BrutalCard className="hover:translate-y-[-4px] transition-transform duration-300">
            <div className="w-12 h-12 bg-red-300 border-2 border-brand-black rounded-lg flex items-center justify-center mb-6 shadow-brutal-sm">
              <Layers size={24} />
            </div>
            <h3 className="text-xl font-bold mb-3">Smart Sessions</h3>
            <p className="text-gray-600 leading-relaxed text-sm md:text-base">
              Supports stateless HTTP clients via Smart Header Mode or Global Mode for persistent shell sessions.
            </p>
          </BrutalCard>

          <BrutalCard className="hover:translate-y-[-4px] transition-transform duration-300">
            <div className="w-12 h-12 bg-purple-300 border-2 border-brand-black rounded-lg flex items-center justify-center mb-6 shadow-brutal-sm">
              <FileCode size={24} />
            </div>
            <h3 className="text-xl font-bold mb-3">Structured Output</h3>
            <p className="text-gray-600 leading-relaxed text-sm md:text-base">
              Tools return clean JSON structures where possible, making it easy for LLMs to parse and reason about system state.
            </p>
          </BrutalCard>

          <BrutalCard className="hover:translate-y-[-4px] transition-transform duration-300">
            <div className="w-12 h-12 bg-green-300 border-2 border-brand-black rounded-lg flex items-center justify-center mb-6 shadow-brutal-sm">
              <Database size={24} />
            </div>
            <h3 className="text-xl font-bold mb-3">File Operations</h3>
            <p className="text-gray-600 leading-relaxed text-sm md:text-base">
              Read, write, edit, and sync files safely with atomic operations and built-in diff support for safe patching.
            </p>
          </BrutalCard>

          <BrutalCard className="hover:translate-y-[-4px] transition-transform duration-300">
            <div className="w-12 h-12 bg-gray-300 border-2 border-brand-black rounded-lg flex items-center justify-center mb-6 shadow-brutal-sm">
              <Terminal size={24} />
            </div>
            <h3 className="text-xl font-bold mb-3">Log Analysis</h3>
            <p className="text-gray-600 leading-relaxed text-sm md:text-base">
              Dedicated tools for searching logs (grep/journalctl) and finding files without overwhelming the context window.
            </p>
          </BrutalCard>
        </div>
      </div>
    </section>
  );
};

export default Features;