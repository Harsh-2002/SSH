import React from 'react';
import Button from '../ui/Button';
import { Github, Terminal } from 'lucide-react';

const Hero: React.FC = () => {
  return (
    <section className="pt-16 pb-12 md:pt-24 md:pb-20 px-4 sm:px-6 lg:px-8 max-w-7xl mx-auto flex flex-col items-center text-center">
      <div className="space-y-6 md:space-y-8 flex flex-col items-center max-w-3xl">
        <h1 className="text-4xl sm:text-5xl md:text-7xl font-bold leading-[0.95] tracking-tight">
          The Bridge Between <br />
          <span className="bg-brand-yellow px-2 inline-block -rotate-1 transform mt-2 border-2 border-brand-black shadow-brutal-sm">AI Agents</span> & <br />
          Infrastructure.
        </h1>
        <p className="text-lg md:text-xl text-gray-600 max-w-xl leading-relaxed">
          Connect your LLMs to remote machines over SSH. High-performance Go server designed for mission-critical stability with zero runtime dependencies.
        </p>
        <div className="pt-4">
          <a href="https://github.com/Harsh-2002/SSH-MCP" target="_blank" rel="noreferrer">
            <Button className="flex items-center gap-3 justify-center bg-brand-yellow text-brand-black hover:bg-yellow-300 w-full sm:w-auto">
              <Github size={20} />
              <span>View Repository</span>
            </Button>
          </a>
        </div>
      </div>

      {/* Visual Decoration */}
      <div className="relative mt-12 md:mt-20 w-full max-w-2xl text-left">
        <div className="absolute inset-0 bg-brand-yellow rounded-2xl border-2 border-brand-black translate-x-2 translate-y-2 sm:translate-x-4 sm:translate-y-4"></div>
        <div className="relative bg-white border-2 border-brand-black rounded-2xl p-4 sm:p-6 md:p-8 shadow-brutal flex flex-col gap-4 md:gap-6">
          <div className="flex items-center gap-4 border-b-2 border-gray-100 pb-4">
            <div className="w-2.5 h-2.5 md:w-3 md:h-3 rounded-full bg-red-500 border border-brand-black"></div>
            <div className="w-2.5 h-2.5 md:w-3 md:h-3 rounded-full bg-yellow-500 border border-brand-black"></div>
            <div className="w-2.5 h-2.5 md:w-3 md:h-3 rounded-full bg-green-500 border border-brand-black"></div>
            <div className="font-mono text-xs md:text-sm text-gray-400 ml-auto flex items-center gap-2">
              <Terminal size={12} className="md:w-[14px] md:h-[14px]" />
              <span>server-1</span>
            </div>
          </div>
          <div className="font-mono text-xs md:text-sm space-y-2 md:space-y-3 overflow-x-auto no-scrollbar">
            <div className="flex gap-2 text-gray-400 whitespace-nowrap">
              <span># Manage infrastructure securely</span>
            </div>
            <div className="flex gap-2 whitespace-nowrap">
              <span className="text-green-600 font-bold">➜</span>
              <span className="text-blue-600 font-bold">~</span>
              <span>connect --host=backend-01</span>
            </div>
            <div className="text-gray-500">Authenticated. Session established.</div>

            <div className="flex gap-2 pt-2 whitespace-nowrap">
              <span className="text-green-600 font-bold">➜</span>
              <span className="text-blue-600 font-bold">~</span>
              <span>mcp run "docker stats --no-stream"</span>
            </div>
            <div className="text-gray-500">
              {'>'} CONTAINER ID   NAME        CPU %     MEM USAGE<br />
              {'>'} a1b2c3d4e5     api-svc     0.05%     45MiB / 1GiB<br />
              {'>'} f9e8d7c6b5     db-pg       1.20%     150MiB / 2GiB<br />
              <span className="text-green-600 font-bold">Command completed.</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Hero;