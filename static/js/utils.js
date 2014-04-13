$(document).ready(function(){
	function Doc(Name, Type, Content, Tags) {
		this.Name = Name;
		this.Type = Type;
		this.Content = Content;
		this.Tags = Tags;
	}

	function getcontent(d) {
		switch(d.Type) {
		case "url":
			return '<a href="'+d.Content+'">'+d.Content+'</a>';
		default:
			return d.Content;
		}
	}

	/* Compile JSON-Doc into xHTML */
	function doc2html(d) {
		s = '<div class="doc panel">';
			s += '<div class="panel-heading"><span class="name">'+d.Name+'</span>';
				s += '<button class="close">&times;</button>'
				s += '<button class="close">^</button>'
				s += '<div class="tags">'+d.Tags.join(", ")+'</div>'
			s += '</div>';
			s += '<div class="panel-body">';
				s += getcontent(d);
			s += '</div>';
		s += '</div>';

		return s;
	}

	/* Add document templating */
	$('#adddoc').prepend(doc2html(new Doc('Name of the doc', 'text', 'Content', ['private'])));
	$("#adddoc>.doc>.panel-heading>.name").attr('contenteditable', 'true');
	$('#adddoc>.doc>.panel-heading>.tags').attr('contenteditable', 'true');
	$('#adddoc>.doc>.panel-body').attr('contenteditable', 'true');

	/* Add document logic */
	$('#addb').click(function() {
		/* XXX Check if element has been modified */
		d = new Doc(
			$('#adddoc>.doc>.panel-heading>.name').text(),
			'text',
			$('#adddoc>.doc>.panel-body').text(),
			$('#adddoc>.doc>.panel-heading>.tags').text());
		alert(d.toSource());
	});

	/* Search logic */
	$('#searchb').click(function() {
		/* clean previous results */
		$('#docs').html('Loading...');

		/* fetch docs */
		$.getJSON('/api/'+$('#searchq').val().replace(/\s+/, '\u001F'), function(ds) {
			if (ds == null || ds.length == 0) {
				$('#docs').html('No results.');
			} else {
				$('#docs').html('');
				$.each(ds, function(k, v) {
					$('#docs').append(doc2html(v))
				});
			}
		});    
	});
	/* call previous when hitting enter from associated input */
	$('#searchq').keypress(function(e){
		if (e.which == 13){ $('#searchb').click(); }
	});
});
