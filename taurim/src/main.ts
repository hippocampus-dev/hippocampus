import { h, render, Component } from 'preact';
import { invoke } from '@tauri-apps/api/core';

class App extends Component {
  constructor(props) {
    super(props);
    this.state = {
      message: '',
      showModal: false
    };
  }

  handleClick = async () => {
    try {
      const response = await invoke('greet', { name: 'Android User' });
      this.setState({ message: response, showModal: true });
    } catch (error) {
      console.error('Error:', error);
      this.setState({ message: `Error: ${error}`, showModal: true });
    }
  };

  closeModal = () => {
    this.setState({ showModal: false });
  };

  render() {
    const { message, showModal } = this.state;

    return h('div', { className: 'container' },
      h('h1', null, '🎉 Hello from Taurim!'),
      h('p', null, 'Minimal Tauri Android App'),
      h('button', {
        onClick: this.handleClick,
        style: {
          padding: '12px 24px',
          fontSize: '16px',
          borderRadius: '8px',
          border: 'none',
          background: 'white',
          color: '#667eea',
          cursor: 'pointer',
          marginTop: '20px'
        }
      }, 'Tap to Greet'),

      showModal && h('div', {
        style: {
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: 'rgba(0, 0, 0, 0.5)',
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          zIndex: 1000
        }
      },
        h('div', {
          style: {
            background: 'white',
            padding: '24px',
            borderRadius: '12px',
            maxWidth: '80%',
            boxShadow: '0 4px 6px rgba(0, 0, 0, 0.1)'
          }
        },
          h('p', {
            style: {
              margin: '0 0 20px 0',
              color: '#333',
              fontSize: '18px'
            }
          }, message),
          h('button', {
            onClick: this.closeModal,
            style: {
              padding: '8px 16px',
              fontSize: '14px',
              borderRadius: '6px',
              border: 'none',
              background: '#667eea',
              color: 'white',
              cursor: 'pointer'
            }
          }, 'Close')
        )
      )
    );
  }
}

render(h(App), document.getElementById('app'));