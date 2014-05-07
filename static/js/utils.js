$(document).ready(function(){
/*	XXX if enable, accordion disable (useful bug?)
	$('.panel-collapse').collapse({
		toggle: false
	})
*/

	$('#add').submit(function() {
		$(this).find("input[name=content]").val($(this).find("#addcol").text())
	return true
	})
})
