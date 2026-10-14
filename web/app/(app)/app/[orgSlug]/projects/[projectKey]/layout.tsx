import type { ReactNode } from 'react';

// Project scope layout: hosts the @modal parallel slot so task links
// intercept into a modal over the board (ADR-018 follow-up). Direct loads and
// refreshes still serve the full task page.
//
// The inline style reserves the scrollbar gutter for the whole project scope so
// switching between short (board) and long (settings) tabs never shifts the page
// by a scrollbar's width. It persists across tab navigations (layouts don't remount).
export default function ProjectLayout({
  children,
  modal,
}: {
  children: ReactNode;
  modal: ReactNode;
}) {
  return (
    <>
      <style>{'html{scrollbar-gutter:stable}'}</style>
      {children}
      {modal}
    </>
  );
}
