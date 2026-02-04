import React, { useState } from 'react';
import CodeBlock from '../ui/CodeBlock';
import { INSTALL_METHODS } from '../../constants';
import { Box, FileText, Terminal } from 'lucide-react';

const Quickstart: React.FC = () => {
  const [activeTab, setActiveTab] = useState(INSTALL_METHODS[0].id);
  const activeMethod = INSTALL_METHODS.find(m => m.id === activeTab) || INSTALL_METHODS[0];

  const getIcon = (id: string) => {
    switch (id) {
      case 'docker': return <Box size={18} />;
      case 'compose': return <FileText size={18} />;
      case 'local': return <Terminal size={18} />;
      default: return <Terminal size={18} />;
    }
  };

  return (
    <section id="quickstart" className="py-12 md:py-24 bg-brand-white relative overflow-hidden">
      {/* Decorative background circle */}
      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[120%] h-[120%] bg-brand-yellow/5 rounded-full blur-3xl -z-10 pointer-events-none"></div>

      <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 relative z-10">
        <div className="text-center mb-10 md:mb-14">
          <h2 className="text-4xl md:text-6xl font-bold mb-6 tracking-tight">
            Start <span className="underline decoration-wavy decoration-brand-yellow decoration-4 underline-offset-4">Building</span>
          </h2>
          <p className="text-lg md:text-xl text-gray-600 max-w-2xl mx-auto font-medium">
            Production-ready configuration for any environment.
          </p>
        </div>

        <div className="max-w-4xl mx-auto">
          {/* Circular Pill Tabs */}
          <div className="flex flex-wrap justify-center gap-3 md:gap-4 mb-8">
            {INSTALL_METHODS.map((method) => (
              <button
                key={method.id}
                onClick={() => setActiveTab(method.id)}
                className={`
                  pl-5 pr-6 py-3 rounded-full font-bold text-sm md:text-base border-2 border-brand-black transition-all duration-300 flex items-center gap-2
                  ${activeTab === method.id
                    ? 'bg-brand-black text-brand-yellow scale-105 shadow-brutal'
                    : 'bg-white text-gray-600 hover:bg-brand-yellow/20 hover:scale-105'
                  }
                `}
              >
                {getIcon(method.id)}
                {method.label}
              </button>
            ))}
          </div>

          {/* Large Card (Consistent Radius) */}
          <div className="bg-white border-2 border-brand-black rounded-xl p-6 md:p-8 shadow-brutal-lg relative overflow-hidden">
            {/* Card Decoration */}
            <div className="absolute top-0 right-0 p-6 opacity-10 pointer-events-none">
              <svg width="100" height="100" viewBox="0 0 100 100" fill="none" xmlns="http://www.w3.org/2000/svg">
                <circle cx="50" cy="50" r="40" stroke="currentColor" strokeWidth="8" />
                <circle cx="50" cy="50" r="20" fill="currentColor" />
              </svg>
            </div>

            <div className="flex flex-col gap-6 relative z-10">
              <div className="flex items-center gap-3 mb-2">
                <div className={`w-3 h-3 rounded-full ${activeTab === 'docker' ? 'bg-blue-500' : activeTab === 'compose' ? 'bg-purple-500' : 'bg-brand-yellow'}`}></div>
                <h3 className="font-bold text-xl md:text-2xl">{activeMethod.label} Setup</h3>
              </div>

              <CodeBlock code={activeMethod.commands} />

              {activeMethod.note && (
                <div className="flex items-start gap-4 p-5 bg-gray-50 border-2 border-brand-black rounded-lg">
                  <span className="text-xl bg-brand-yellow w-8 h-8 rounded-full flex items-center justify-center border border-brand-black flex-shrink-0">i</span>
                  <div>
                    <p className="text-sm md:text-base font-medium text-gray-800 pt-1 leading-relaxed">{activeMethod.note}</p>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Quickstart;