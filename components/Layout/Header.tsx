import React, { useState } from 'react';
import { Menu, X, Terminal, Github } from 'lucide-react';
import { NAV_ITEMS } from '../../constants';
import Button from '../ui/Button';

const Header: React.FC = () => {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <header className="fixed top-0 left-0 right-0 z-50 bg-brand-white/90 backdrop-blur-md border-b-2 border-brand-black">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between items-center h-20">
          {/* Logo */}
          <div className="flex items-center gap-3 group cursor-pointer" onClick={() => window.scrollTo(0,0)}>
            <div className="bg-brand-yellow p-2 border-2 border-brand-black rounded-lg shadow-brutal-sm group-hover:rotate-6 transition-transform duration-300">
              <Terminal size={24} className="text-brand-black" />
            </div>
            <span className="font-bold text-xl tracking-tight group-hover:text-brand-yellow group-hover:bg-brand-black group-hover:px-2 group-hover:-ml-2 transition-all duration-300 rounded">SSH MCP</span>
          </div>

          {/* Desktop Nav */}
          <nav className="hidden md:flex items-center gap-8">
            {NAV_ITEMS.map((item) => (
              <a 
                key={item.label} 
                href={item.href}
                className="text-brand-black font-medium hover:text-brand-yellow hover:bg-brand-black px-3 py-1 rounded transition-all duration-200 hover:-translate-y-0.5 hover:shadow-[2px_2px_0px_0px_rgba(18,18,18,1)] border border-transparent hover:border-brand-black"
              >
                {item.label}
              </a>
            ))}
            <a href="https://github.com/Harsh-2002/SSH-MCP" target="_blank" rel="noreferrer">
              <Button variant="secondary" className="!py-2 !px-4 text-sm flex items-center gap-2">
                <Github size={16} />
                GitHub
              </Button>
            </a>
          </nav>

          {/* Mobile Menu Toggle */}
          <div className="md:hidden">
            <button 
              onClick={() => setIsOpen(!isOpen)}
              className="p-2 border-2 border-brand-black rounded bg-white hover:bg-gray-100 active:bg-brand-yellow transition-colors"
            >
              {isOpen ? <X size={24} /> : <Menu size={24} />}
            </button>
          </div>
        </div>
      </div>

      {/* Mobile Nav */}
      {isOpen && (
        <div className="md:hidden bg-white border-b-2 border-brand-black absolute w-full px-4 py-6 flex flex-col gap-4 shadow-xl animate-in slide-in-from-top-5 duration-200">
          {NAV_ITEMS.map((item) => (
            <a 
              key={item.label} 
              href={item.href}
              onClick={() => setIsOpen(false)}
              className="text-xl font-bold border-b-2 border-gray-100 pb-2 hover:pl-4 hover:border-l-4 hover:border-l-brand-yellow hover:bg-gray-50 transition-all duration-200"
            >
              {item.label}
            </a>
          ))}
          <a href="https://github.com/Harsh-2002/SSH-MCP" target="_blank" rel="noreferrer">
            <Button fullWidth>View on GitHub</Button>
          </a>
        </div>
      )}
    </header>
  );
};

export default Header;