/**
 * @jest-environment jsdom
 */
import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import ChatInput from './ChatInput'

describe('ChatInput', () => {
  const noop = () => {}

  it('renders textarea with placeholder', () => {
    render(<ChatInput value="" onChange={noop} onSubmit={noop} />)
    expect(screen.getByPlaceholderText(/Ask Prof Nova/)).toBeInTheDocument()
  })

  it('calls onChange when user types', () => {
    const onChange = jest.fn()
    render(<ChatInput value="" onChange={onChange} onSubmit={noop} />)
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'hello' } })
    expect(onChange).toHaveBeenCalledWith('hello')
  })

  it('calls onSubmit when Enter is pressed with non-empty value', () => {
    const onSubmit = jest.fn()
    render(<ChatInput value="hello" onChange={noop} onSubmit={onSubmit} />)
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', shiftKey: false })
    expect(onSubmit).toHaveBeenCalledTimes(1)
  })

  it('does not call onSubmit on Shift+Enter', () => {
    const onSubmit = jest.fn()
    render(<ChatInput value="hello" onChange={noop} onSubmit={onSubmit} />)
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', shiftKey: true })
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('does not call onSubmit when value is empty whitespace', () => {
    const onSubmit = jest.fn()
    render(<ChatInput value="   " onChange={noop} onSubmit={onSubmit} />)
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', shiftKey: false })
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('disables textarea when disabled=true', () => {
    render(<ChatInput value="hi" onChange={noop} onSubmit={noop} disabled />)
    expect(screen.getByRole('textbox')).toBeDisabled()
  })

  it('disables send button when value is empty', () => {
    render(<ChatInput value="" onChange={noop} onSubmit={noop} />)
    expect(screen.getByRole('button', { name: /Send/i })).toBeDisabled()
  })

  it('disables send button when disabled=true', () => {
    render(<ChatInput value="hello" onChange={noop} onSubmit={noop} disabled />)
    expect(screen.getByRole('button', { name: /Send/i })).toBeDisabled()
  })

  it('enables send button when value is non-empty and not disabled', () => {
    render(<ChatInput value="hello" onChange={noop} onSubmit={noop} />)
    expect(screen.getByRole('button', { name: /Send/i })).not.toBeDisabled()
  })

  it('calls onSubmit on form submit', () => {
    const onSubmit = jest.fn()
    const { container } = render(<ChatInput value="hello" onChange={noop} onSubmit={onSubmit} />)
    fireEvent.submit(container.querySelector('form')!)
    expect(onSubmit).toHaveBeenCalledTimes(1)
  })
})
