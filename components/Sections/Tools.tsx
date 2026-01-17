import React from 'react';
import { TOOL_CATEGORIES } from '../../constants';

const Tools: React.FC = () => {
  return (
    <section id="tools" className="py-12 md:py-20 max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div className="text-center mb-10 md:mb-16">
        <h2 className="text-3xl md:text-4xl font-bold">Tool Reference</h2>
        <p className="mt-4 text-gray-600 text-sm md:text-base">All tools accept a <code>target</code> parameter (default: "primary").</p>
      </div>

      <div className="grid lg:grid-cols-2 gap-6 md:gap-8">
        {TOOL_CATEGORIES.map((category) => (
          <div key={category.title} className="bg-white border-2 border-brand-black shadow-brutal rounded-xl overflow-hidden hover:shadow-brutal-lg transition-shadow duration-300">
            <div className="bg-brand-black text-white px-4 py-3 md:px-6 md:py-4 font-mono font-bold border-b-2 border-brand-black flex justify-between items-center text-sm md:text-base">
              <span>{category.title}</span>
              <span className="text-xs bg-brand-yellow text-brand-black px-2 py-1 rounded border border-white/20">
                {category.tools.length} Tools
              </span>
            </div>
            <div className="divide-y-2 divide-gray-100">
              {category.tools.map((tool) => (
                <div key={tool.name} className="p-4 hover:bg-brand-yellow/5 transition-colors duration-200 group cursor-default">
                  <code className="block text-xs md:text-sm font-bold text-blue-700 bg-blue-50 w-fit px-2 py-0.5 rounded mb-2 font-mono break-all group-hover:bg-blue-100 transition-colors">
                    {tool.name}
                  </code>
                  <p className="text-xs md:text-sm text-gray-700">{tool.description}</p>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
};

export default Tools;