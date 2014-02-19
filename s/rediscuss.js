/** @jsx React.DOM */
var Rediscuss = React.createClass({
  render: function() {
    return <div />;
  }
});
React.renderComponent(
  <Rediscuss res={location.hash.substring(1)} />,
  document.getElementById('rediscuss')
);
