import React from 'react';
import { CONFIG_ITEMS } from '../../constants';

const Configuration: React.FC = () => {
  return (
    <section id="configuration" className="py-12 md:py-20 bg-white border-t-2 border-brand-black">
      <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8">
        <h2 className="text-3xl md:text-4xl font-bold mb-8 md:mb-10 text-center">Configuration</h2>
        
        <div className="overflow-hidden border-2 border-brand-black rounded-xl shadow-brutal bg-white hover:shadow-brutal-lg transition-shadow duration-300">
          <div className="overflow-x-auto no-scrollbar">
            <table className="min-w-full divide-y-2 divide-brand-black">
              <thead className="bg-brand-yellow">
                <tr className="divide-x-2 divide-brand-black">
                  <th scope="col" className="px-4 py-3 md:px-6 md:py-4 text-left text-xs font-bold text-brand-black uppercase tracking-wider font-mono min-w-[120px]">Variable</th>
                  <th scope="col" className="px-4 py-3 md:px-6 md:py-4 text-left text-xs font-bold text-brand-black uppercase tracking-wider font-mono whitespace-nowrap">Type</th>
                  <th scope="col" className="px-4 py-3 md:px-6 md:py-4 text-left text-xs font-bold text-brand-black uppercase tracking-wider font-mono min-w-[100px]">Default</th>
                  <th scope="col" className="px-4 py-3 md:px-6 md:py-4 text-left text-xs font-bold text-brand-black uppercase tracking-wider font-mono min-w-[200px]">Description</th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y-2 divide-gray-100">
                {CONFIG_ITEMS.map((item, idx) => (
                  <tr key={item.variable} className={`divide-x-2 divide-gray-100 transition-colors duration-150 ${idx % 2 === 1 ? 'bg-gray-50/50 hover:bg-brand-yellow/10' : 'hover:bg-brand-yellow/10'}`}>
                    <td className="px-4 py-3 md:px-6 md:py-4 text-xs md:text-sm font-bold font-mono text-brand-black align-top break-words">
                      {item.variable}
                    </td>
                    <td className="px-4 py-3 md:px-6 md:py-4 text-xs font-mono text-gray-500 align-top whitespace-nowrap">
                      {item.type}
                    </td>
                    <td className="px-4 py-3 md:px-6 md:py-4 text-xs md:text-sm font-mono text-gray-600 bg-gray-50 align-top break-words">
                      {item.default}
                    </td>
                    <td className="px-4 py-3 md:px-6 md:py-4 text-xs md:text-sm text-gray-700 align-top">
                      {item.description}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Configuration;