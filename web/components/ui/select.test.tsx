import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { afterEach, describe, expect, it } from 'vitest';

import { Select } from './select';

const OPTIONS = [
  { value: '', label: 'Any' },
  { value: 'a', label: 'Alpha' },
  { value: 'b', label: 'Beta' },
];

function Harness({ initial = '' }: { initial?: string }) {
  const [v, setV] = useState(initial);
  return <Select aria-label="Pick" value={v} onValueChange={setV} options={OPTIONS} />;
}

afterEach(cleanup);

describe('Select', () => {
  it('shows the selected label and opens a listbox', () => {
    render(<Harness initial="a" />);
    const trigger = screen.getByRole('combobox', { name: 'Pick' });
    expect(trigger).toHaveTextContent('Alpha');
    fireEvent.click(trigger);
    expect(screen.getAllByRole('option')).toHaveLength(3);
    expect(screen.getByRole('option', { name: /Alpha/ })).toHaveAttribute('aria-selected', 'true');
  });

  it('selects with the keyboard and closes', () => {
    render(<Harness />);
    const trigger = screen.getByRole('combobox', { name: 'Pick' });
    fireEvent.keyDown(trigger, { key: 'ArrowDown' });
    const list = screen.getByRole('listbox');
    fireEvent.keyDown(list, { key: 'ArrowDown' });
    fireEvent.keyDown(list, { key: 'Enter' });
    expect(trigger).toHaveTextContent('Alpha');
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('closes on Escape without changing the value', () => {
    render(<Harness initial="b" />);
    fireEvent.click(screen.getByRole('combobox'));
    fireEvent.keyDown(screen.getByRole('listbox'), { key: 'Escape' });
    expect(screen.queryByRole('listbox')).toBeNull();
    expect(screen.getByRole('combobox')).toHaveTextContent('Beta');
  });
});
