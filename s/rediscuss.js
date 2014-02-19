/** @jsx React.DOM */
var Rediscuss = React.createClass({
  render: function() {
    return <div></div>;
  }
});


function rediscuss(options) {
  React.renderComponent(
    <Rediscuss options={options} />,
    document.getElementById('rediscuss')
  );
}
