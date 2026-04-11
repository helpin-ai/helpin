// Wait for the DOM to be fully loaded
document.addEventListener('DOMContentLoaded', () => {
  const checkHelpinLoaded = () => {
    const helpin = (window as any).helpin;

    if (!helpin) {
      console.log('Helpin SDK not loaded yet, retrying in 100ms');
      setTimeout(checkHelpinLoaded, 100);
      return;
    }

    console.log('Helpin SDK loaded successfully', helpin);

    // Test track event
    document.getElementById('trackEvent')?.addEventListener('click', () => {
      helpin('track', 'button_click', { buttonId: 'trackEvent' });
      console.log('Track event sent');
    });

    // Test identify user
    document.getElementById('identifyUser')?.addEventListener('click', () => {
      helpin('id', {
        id: 'user123',
        email: 'test@example.com',
        first_name: 'Test',
        last_name: 'User',
        custom: {},

        company: {
          id: 'company123',
          name: 'Test Company',
          created_at: '2023-01-01',
        },
      });
      console.log('User identified');
    });

    console.log('Helpin SDK test scripts loaded');
  };

  checkHelpinLoaded();
});
