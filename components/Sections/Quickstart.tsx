import React, { useState } from 'react';
import BrutalCard from '../ui/BrutalCard';
import CodeBlock from '../ui/CodeBlock';
import { INSTALL_METHODS } from '../../constants';

const Quickstart: React.FC = () => {
  const [activeTab, setActiveTab] = useState(INSTALL_METHODS[0].id);

  const activeMethod = INSTALL_METHODS.find(m => m.id === activeTab) || INSTALL_METHODS[0];

  return (
    <section id="quickstart" className="py-12 md:py-16 bg-brand-white">
      <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-8 md:mb-10">
          <h2 className="text-3xl md:text-4xl font-bold mb-4 bg-brand-yellow inline-block px-4 py-1 border-2 border-brand-black shadow-brutal-sm -rotate-1">Quickstart</h2>
          <p className="text-lg md:text-xl text-gray-600 mt-4 md:mt-6">Choose your preferred deployment method.</p>
        </div>

        <BrutalCard noPadding>
          <div className="flex flex-wrap border-b-2 border-brand-black bg-gray-50 justify-center">
            {INSTALL_METHODS.map((method) => (
              <button
                key={method.id}
                onClick={() => setActiveTab(method.id)}
                className={`px-4 py-3 md:px-6 md:py-4 font-mono font-bold text-xs sm:text-sm md:text-base border-r-0 sm:border-r-2 border-brand-black last:border-r-0 transition-all duration-200 flex-grow sm:flex-grow-0 ${activeTab === method.id
                    ? 'bg-brand-black text-brand-yellow'
                    : 'bg-transparent text-gray-500 hover:bg-brand-yellow/20 hover:text-brand-black'
                  }`}
              >
                {method.label}
              </button>
            ))}
          </div>

          <div className="p-4 sm:p-6 md:p-8 bg-white flex flex-col gap-6">
            <CodeBlock code={activeMethod.commands} />

            {activeMethod.note && (
              <div className="bg-yellow-50 border-2 border-brand-black p-4 rounded-lg flex gap-3 items-start transition-transform hover:scale-[1.01] duration-300">
                <p className="text-xs md:text-sm font-medium pt-1">{activeMethod.note}</p>
              </div>
            )}
          </div>
        </BrutalCard>
      </div>
    </section>
  );
};

export default Quickstart;