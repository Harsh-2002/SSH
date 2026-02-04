import React from 'react';
import screenshot1 from '../../assets/SCR-20260116-nfoe.avif';
import screenshot2 from '../../assets/SCR-20260116-nhbs.avif';

const Screenshots: React.FC = () => {
  const screenshots = [
    { src: screenshot1, alt: 'SSH MCP Server Connection Management' },
    { src: screenshot2, alt: 'SSH MCP Server DevOps Tools' },
  ];

  return (
    <section id="screenshots" className="py-12 md:py-20 bg-brand-white">
      <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-10 md:mb-16">
          <h2 className="text-3xl md:text-4xl font-bold mb-4">See It In Action</h2>
          <p className="text-lg md:text-xl text-gray-600 max-w-2xl mx-auto">
            Real screenshots showing SSH MCP Server managing remote infrastructure, executing commands, and interacting with AI agents.
          </p>
        </div>

        <div className="flex flex-col gap-8 md:gap-12">
          {screenshots.map((screenshot, index) => (
            <div key={index} className="w-full relative">
              <div className="absolute inset-0 bg-brand-yellow rounded-lg translate-x-2 translate-y-2 sm:translate-x-4 sm:translate-y-4"></div>
              <div className="relative bg-white border-2 border-brand-black rounded-lg shadow-brutal">
                <img
                  src={screenshot.src}
                  alt={screenshot.alt}
                  className="w-full h-auto rounded-lg"
                  loading="lazy"
                />
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default Screenshots;
