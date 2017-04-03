$(document).ready(function () {
  $("#repo").change(function(){
    console.log($("#repo").val())
  });
  $("button").click(function() {
    console.log($("#repo").val())
  });
  $("form").submit(function(){
    console.log($("#repo").val())
  });
});
