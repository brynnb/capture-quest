import React, { createContext, useContext, useRef, ReactNode } from 'react';

interface LayoutContextType {
    mainRef: React.RefObject<HTMLDivElement>;
}

const LayoutContext = createContext<LayoutContextType | null>(null);

export const LayoutProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
    const mainRef = useRef<HTMLDivElement>(null);
    return (
        <LayoutContext.Provider value={{ mainRef }}>
            {children}
        </LayoutContext.Provider>
    );
};

export const useLayout = () => {
    const context = useContext(LayoutContext);
    if (!context) {
        throw new Error('useLayout must be used within a LayoutProvider');
    }
    return context;
};
