import React from 'react';
import Hero from './components/Sections/Hero';
import Quickstart from './components/Sections/Quickstart';
import Features from './components/Sections/Features';
import Tools from './components/Sections/Tools';
import Screenshots from './components/Sections/Screenshots';
import Configuration from './components/Sections/Configuration';
import Footer from './components/Layout/Footer';

const App: React.FC = () => {
  return (
    <div className="min-h-screen font-sans selection:bg-brand-yellow selection:text-brand-black bg-brand-white flex flex-col overflow-x-hidden">
      <main className="flex-grow">
        <Hero />
        <Features />
        <Quickstart />
        <Screenshots />
        <Tools />
        <Configuration />
      </main>
      <Footer />
    </div>
  );
};

export default App;