import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import StatusTag from './StatusTag';

describe('StatusTag', () => {
  it.each([
    ['SUCCESS', 'Success'],
    ['FAILED', 'Failed'],
    ['RUNNING', 'Running'],
    ['PENDING', 'Pending'],
    ['CANCELLED', 'Cancelled'],
    ['PREPARING', 'Preparing'],
  ])('renders %s as %s', (status, label) => {
    render(<StatusTag status={status as never} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });
});
