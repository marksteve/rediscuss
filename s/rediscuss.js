/** @jsx React.DOM */

var Auth = React.createClass({
  register: function() {
    var creds = {
      name: this.refs.name.getDOMNode().value.trim(),
      password: this.refs.password.getDOMNode().value
    };
    this.props.onRegister(creds);
  },
  render: function() {
    return (
      <div className="auth">
        <p><input ref="name" type="text" placeholder="Username" /></p>
        <p><input ref="password" type="password" placeholder="Password" /></p>
        <p>
          <button onClick={this.login}>Login</button>
          or
          <button onClick={this.register}>Register</button>
        </p>
      </div>
    );
  }
});

var Rediscuss = React.createClass({
  getInitialState: function() {
    return {
      message: null,
      token: null
    };
  },
  register: function(creds) {
    var self = this;
    superagent
      .post(self.props.options.root + '/register')
      .send(creds)
      .end(function(res) {
        if (res.error) {
          self.setState({
            message: res.body.message
          });
        }
        self.setState({
          token: res.body.token
        });
      });
    self.setState({
      message: null
    });
  },
  render: function() {
    var nodes = [];
    if (!this.props.options.token) {
      nodes.push(
        <Auth onRegister={this.register} />
      );
    }
    if (this.state.message) {
      nodes.push(
        <div className="message">
          <p>{this.state.message}</p>
        </div>
      );
    }
    return (
      <div>{nodes}</div>
    );
  }
});


function rediscuss(options) {
  React.renderComponent(
    <Rediscuss options={options} />,
    document.getElementById('rediscuss')
  );
}
